package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"github.com/kataras/golog"
)

const (
	KeyTokenSecret = "token_secret"
	KeyAppConfig   = "app_config"
)

// PushChannel 钉钉 / 飞书 / 企业微信 webhook，独立落库便于查看和热更新。
type PushChannel struct {
	Channel     string `json:"channel"`
	Enabled     bool   `json:"enabled"`
	AccessToken string `json:"access_token"`
	SignSecret  string `json:"sign_secret"`
	ExtraKey    string `json:"extra_key"`
}

const (
	RoleAdmin    = "admin"
	RoleReadonly = "readonly"
)

type User struct {
	Username     string
	PasswordHash string
	Role         string
}

func NormalizeRole(role string) string {
	if strings.TrimSpace(role) == RoleReadonly {
		return RoleReadonly
	}
	return RoleAdmin
}

type AppConfig struct {
	Sources         []string            `json:"sources"`
	Interval        string              `json:"interval"`
	EnableCVEFilter bool                `json:"enable_cve_filter"`
	NoGithubSearch  bool                `json:"no_github_search"`
	NoStartMessage  bool                `json:"no_start_message"`
	NoSleep         bool                `json:"no_sleep"`
	NoFilter        bool                `json:"no_filter"`
	WhiteKeywords   []string            `json:"white_keywords"`
	BlackKeywords   []string            `json:"black_keywords"`
	Proxy           string              `json:"proxy"`
	SkipTLSVerify   bool                `json:"skip_tls_verify"`
	Pusher          []map[string]string `json:"pusher"`
}

type Store struct {
	db      *sql.DB
	dialect string
	log     *golog.Logger
}

func New(db *sql.DB, dialectName string) (*Store, error) {
	s := &Store{
		db:      db,
		dialect: dialectName,
		log:     golog.Child("[store]"),
	}
	if err := s.initTables(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) initTables() error {
	users := `
CREATE TABLE IF NOT EXISTS web_users (
  username VARCHAR(64) PRIMARY KEY,
  password_hash TEXT NOT NULL,
  role VARCHAR(16) NOT NULL DEFAULT 'admin',
  create_time TEXT NOT NULL
)`
	settings := `
CREATE TABLE IF NOT EXISTS web_settings (
  k VARCHAR(64) PRIMARY KEY,
  v TEXT NOT NULL,
  update_time TEXT NOT NULL
)`
	if _, err := s.db.Exec(users); err != nil {
		return fmt.Errorf("create web_users: %w", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE web_users ADD COLUMN role VARCHAR(16) NOT NULL DEFAULT 'admin'`); err != nil {
		s.log.Debugf("web_users.role 列已存在或无需迁移: %v", err)
	}
	pushers := `
CREATE TABLE IF NOT EXISTS web_pushers (
  channel VARCHAR(32) PRIMARY KEY,
  enabled INTEGER NOT NULL DEFAULT 0,
  access_token TEXT NOT NULL DEFAULT '',
  sign_secret TEXT NOT NULL DEFAULT '',
  extra_key TEXT NOT NULL DEFAULT '',
  update_time TEXT NOT NULL
)`
	if _, err := s.db.Exec(settings); err != nil {
		return fmt.Errorf("create web_settings: %w", err)
	}
	if _, err := s.db.Exec(pushers); err != nil {
		return fmt.Errorf("create web_pushers: %w", err)
	}
	s.log.Infof("sqlite 元数据表已就绪: web_users / web_settings / web_pushers")
	return nil
}

func (s *Store) Get(key string) (string, bool, error) {
	var val string
	err := s.db.QueryRow(`SELECT v FROM web_settings WHERE k = `+s.ph(1), key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (s *Store) Set(key, val string) error {
	now := time.Now().Format(time.RFC3339)
	var err error
	switch s.dialect {
	case dialect.MySQL:
		_, err = s.db.Exec(`INSERT INTO web_settings(k,v,update_time) VALUES(?,?,?) ON DUPLICATE KEY UPDATE v=VALUES(v), update_time=VALUES(update_time)`, key, val, now)
	case dialect.Postgres:
		_, err = s.db.Exec(`INSERT INTO web_settings(k,v,update_time) VALUES($1,$2,$3) ON CONFLICT(k) DO UPDATE SET v=EXCLUDED.v, update_time=EXCLUDED.update_time`, key, val, now)
	default:
		_, err = s.db.Exec(`INSERT INTO web_settings(k,v,update_time) VALUES(?,?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v, update_time=excluded.update_time`, key, val, now)
	}
	return err
}

func (s *Store) LoadPushChannels() ([]PushChannel, bool, error) {
	rows, err := s.db.Query(`SELECT channel, enabled, access_token, sign_secret, extra_key FROM web_pushers ORDER BY channel`)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var items []PushChannel
	for rows.Next() {
		var item PushChannel
		var enabled int
		if err = rows.Scan(&item.Channel, &enabled, &item.AccessToken, &item.SignSecret, &item.ExtraKey); err != nil {
			return nil, false, err
		}
		item.Enabled = enabled != 0
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	if len(items) == 0 {
		return nil, false, nil
	}
	return items, true, nil
}

func (s *Store) SavePushChannels(items []PushChannel) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Format(time.RFC3339)
	for _, item := range items {
		enabled := 0
		if item.Enabled {
			enabled = 1
		}
		var execErr error
		switch s.dialect {
		case dialect.MySQL:
			_, execErr = tx.Exec(
				`INSERT INTO web_pushers(channel,enabled,access_token,sign_secret,extra_key,update_time)
VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE
enabled=VALUES(enabled), access_token=VALUES(access_token), sign_secret=VALUES(sign_secret),
extra_key=VALUES(extra_key), update_time=VALUES(update_time)`,
				item.Channel, enabled, item.AccessToken, item.SignSecret, item.ExtraKey, now,
			)
		case dialect.Postgres:
			_, execErr = tx.Exec(
				`INSERT INTO web_pushers(channel,enabled,access_token,sign_secret,extra_key,update_time)
VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(channel) DO UPDATE SET
enabled=EXCLUDED.enabled, access_token=EXCLUDED.access_token, sign_secret=EXCLUDED.sign_secret,
extra_key=EXCLUDED.extra_key, update_time=EXCLUDED.update_time`,
				item.Channel, enabled, item.AccessToken, item.SignSecret, item.ExtraKey, now,
			)
		default:
			_, execErr = tx.Exec(
				`INSERT INTO web_pushers(channel,enabled,access_token,sign_secret,extra_key,update_time)
VALUES(?,?,?,?,?,?) ON CONFLICT(channel) DO UPDATE SET
enabled=excluded.enabled, access_token=excluded.access_token, sign_secret=excluded.sign_secret,
extra_key=excluded.extra_key, update_time=excluded.update_time`,
				item.Channel, enabled, item.AccessToken, item.SignSecret, item.ExtraKey, now,
			)
		}
		if execErr != nil {
			return execErr
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.log.Infof("推送 webhook 已写入 sqlite, 渠道数=%d", len(items))
	return nil
}

func (s *Store) LoadAppConfig() (*AppConfig, bool, error) {
	raw, ok, err := s.Get(KeyAppConfig)
	if err != nil || !ok || strings.TrimSpace(raw) == "" {
		return nil, false, err
	}
	var cfg AppConfig
	if err = json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, false, err
	}
	return &cfg, true, nil
}

func (s *Store) SaveAppConfig(cfg *AppConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err = s.Set(KeyAppConfig, string(data)); err != nil {
		return err
	}
	s.log.Infof("应用配置已写入 sqlite")
	return nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT username, password_hash, COALESCE(role, 'admin') FROM web_users ORDER BY create_time ASC, username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.Username, &u.PasswordHash, &u.Role); err != nil {
			return nil, err
		}
		u.Role = NormalizeRole(u.Role)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) FindUser(username string) (*User, error) {
	var u User
	err := s.db.QueryRow(`SELECT username, password_hash, COALESCE(role, 'admin') FROM web_users WHERE username = `+s.ph(1), username).Scan(&u.Username, &u.PasswordHash, &u.Role)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Role = NormalizeRole(u.Role)
	return &u, nil
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM web_users WHERE COALESCE(role, 'admin') = `+s.ph(1), RoleAdmin).Scan(&n)
	return n, err
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM web_users`).Scan(&n)
	return n, err
}

func (s *Store) AddUser(username, passwordHash, role string) error {
	_, err := s.db.Exec(
		`INSERT INTO web_users(username, password_hash, role, create_time) VALUES(`+s.placeholders(4)+`)`,
		username, passwordHash, NormalizeRole(role), time.Now().Format(time.RFC3339),
	)
	return err
}

func (s *Store) UpdatePassword(username, passwordHash string) error {
	res, err := s.db.Exec(`UPDATE web_users SET password_hash = `+s.ph(1)+` WHERE username = `+s.ph(2), passwordHash, username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("账号不存在")
	}
	return nil
}

func (s *Store) DeleteUser(username string) error {
	_, err := s.db.Exec(`DELETE FROM web_users WHERE username = `+s.ph(1), username)
	return err
}

func (s *Store) ph(i int) string {
	if s.dialect == dialect.Postgres {
		return fmt.Sprintf("$%d", i)
	}
	return "?"
}

func (s *Store) placeholders(n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = s.ph(i + 1)
	}
	return strings.Join(parts, ",")
}
