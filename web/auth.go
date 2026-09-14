package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kataras/golog"
	"lousentry/store"
	"golang.org/x/crypto/bcrypt"
)

const tokenTTL = 24 * time.Hour

type UserDTO struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AuthStore struct {
	mu          sync.RWMutex
	db          *store.Store
	tokenSecret string
	failures    map[string][]time.Time
	log         *golog.Logger
}

func NewAuthStore(db *store.Store, username, password string) (*AuthStore, error) {
	if db == nil {
		return nil, fmt.Errorf("sqlite store is required")
	}
	if username == "" {
		username = "admin"
	}
	s := &AuthStore{
		db:       db,
		failures: map[string][]time.Time{},
		log:      golog.Child("[auth]"),
	}
	if err := s.ensureSecret(); err != nil {
		return nil, err
	}
	n, err := db.CountUsers()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		hash, err := hashPassword(firstNonEmpty(password, "admin123"))
		if err != nil {
			return nil, err
		}
		if err = db.AddUser(username, hash, store.RoleAdmin); err != nil {
			return nil, err
		}
		s.log.Infof("已在 sqlite 中创建默认账号 %s", username)
	} else if password != "" {
		hash, err := hashPassword(password)
		if err != nil {
			return nil, err
		}
		if existing, _ := db.FindUser(username); existing != nil {
			if err = db.UpdatePassword(username, hash); err != nil {
				return nil, err
			}
		} else if err = db.AddUser(username, hash, store.RoleAdmin); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *AuthStore) ensureSecret() error {
	raw, ok, err := s.db.Get(store.KeyTokenSecret)
	if err != nil {
		return err
	}
	if ok && raw != "" {
		s.tokenSecret = raw
		return nil
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	s.tokenSecret = hex.EncodeToString(secret)
	return s.db.Set(store.KeyTokenSecret, s.tokenSecret)
}

func (s *AuthStore) ListUsers() []UserDTO {
	users, err := s.db.ListUsers()
	if err != nil {
		s.log.Errorf("列出账号失败: %v", err)
		return nil
	}
	out := make([]UserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, UserDTO{Username: u.Username, Role: store.NormalizeRole(u.Role)})
	}
	return out
}

func (s *AuthStore) UserOf(username string) UserDTO {
	user, err := s.db.FindUser(username)
	if err != nil || user == nil {
		return UserDTO{Username: username, Role: store.RoleReadonly}
	}
	return UserDTO{Username: user.Username, Role: store.NormalizeRole(user.Role)}
}

func (s *AuthStore) IsAdmin(username string) bool {
	return s.UserOf(username).Role == store.RoleAdmin
}

func (s *AuthStore) AddUser(username, password, role string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if len(username) < 2 || len(username) > 32 {
		return fmt.Errorf("用户名长度需要 2-32 位")
	}
	if len(password) < 6 {
		return fmt.Errorf("密码至少 6 位")
	}
	exist, err := s.db.FindUser(username)
	if err != nil {
		return err
	}
	if exist != nil {
		return fmt.Errorf("用户名已存在")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	role = store.NormalizeRole(role)
	if err = s.db.AddUser(username, hash, role); err != nil {
		return err
	}
	s.log.Infof("sqlite 新增账号 %s role=%s", username, role)
	return nil
}

func (s *AuthStore) DeleteUser(actor, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if username == actor {
		return fmt.Errorf("不能删除当前登录账号")
	}
	n, err := s.db.CountUsers()
	if err != nil {
		return err
	}
	if n <= 1 {
		return fmt.Errorf("至少保留一个账号")
	}
	exist, err := s.db.FindUser(username)
	if err != nil {
		return err
	}
	if exist == nil {
		return fmt.Errorf("账号不存在")
	}
	if store.NormalizeRole(exist.Role) == store.RoleAdmin {
		admins, err := s.db.CountAdmins()
		if err != nil {
			return err
		}
		if admins <= 1 {
			return fmt.Errorf("至少保留一个管理员账号")
		}
	}
	if err = s.db.DeleteUser(username); err != nil {
		return err
	}
	s.log.Infof("sqlite 删除账号 %s", username)
	return nil
}

func (s *AuthStore) allowLogin(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	list := s.failures[ip]
	var recent []time.Time
	for _, t := range list {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	s.failures[ip] = recent
	return len(recent) < 8
}

func (s *AuthStore) failLogin(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[ip] = append(s.failures[ip], time.Now())
}

func (s *AuthStore) Login(username, password, ip string) (string, error) {
	if !s.allowLogin(ip) {
		return "", fmt.Errorf("尝试次数过多，请稍后再试")
	}
	user, err := s.db.FindUser(username)
	if err != nil {
		return "", err
	}
	if user == nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		s.failLogin(ip)
		return "", fmt.Errorf("用户名或密码错误")
	}
	return s.sign(username), nil
}

func (s *AuthStore) ChangePassword(username, oldPwd, newPwd string) error {
	if len(newPwd) < 6 {
		return fmt.Errorf("新密码至少 6 位")
	}
	user, err := s.db.FindUser(username)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("账号不存在")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPwd)) != nil {
		return fmt.Errorf("原密码不正确")
	}
	hash, err := hashPassword(newPwd)
	if err != nil {
		return err
	}
	return s.db.UpdatePassword(username, hash)
}

func (s *AuthStore) Verify(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	user, err1 := decodePart(parts[0])
	expRaw, err2 := decodePart(parts[1])
	sig, err3 := hex.DecodeString(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", false
	}
	exp, err := strconv.ParseInt(string(expRaw), 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	exist, err := s.db.FindUser(string(user))
	if err != nil || exist == nil {
		return "", false
	}
	s.mu.RLock()
	secret := s.tokenSecret
	s.mu.RUnlock()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(string(user) + "|" + string(expRaw)))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", false
	}
	return string(user), true
}

func (s *AuthStore) sign(username string) string {
	exp := strconv.FormatInt(time.Now().Add(tokenTTL).Unix(), 10)
	s.mu.RLock()
	secret := s.tokenSecret
	s.mu.RUnlock()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(username + "|" + exp))
	return encodePart([]byte(username)) + "." + encodePart([]byte(exp)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func encodePart(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodePart(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
