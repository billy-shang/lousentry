package web

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/golog"
	"lousentry/ctrl"
)

type Server struct {
	app    *ctrl.App
	auth   *AuthStore
	logs   *LogBuffer
	static fs.FS
	log    *golog.Logger
}

func New(app *ctrl.App, auth *AuthStore, logs *LogBuffer, static fs.FS) *Server {
	return &Server{
		app:    app,
		auth:   auth,
		logs:   logs,
		static: static,
		log:    golog.Child("[web]"),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/me", s.authRequired(s.handleMe))
	mux.HandleFunc("/api/password", s.authRequired(s.handlePassword))
	mux.HandleFunc("/api/users", s.authRequired(s.handleUsers))
	mux.HandleFunc("/api/users/", s.authRequired(s.handleUserItem))
	mux.HandleFunc("/api/stats", s.authRequired(s.handleStats))
	mux.HandleFunc("/api/vulns", s.authRequired(s.handleVulns))
	mux.HandleFunc("/api/vulns/", s.authRequired(s.handleVulnItem))
	mux.HandleFunc("/api/vulns/push-unpushed", s.authRequired(s.handlePushUnpushed))
	mux.HandleFunc("/api/report/daily", s.authRequired(s.handleDailyReport))
	mux.HandleFunc("/api/check", s.authRequired(s.handleCheck))
	mux.HandleFunc("/api/push/test", s.authRequired(s.handleTestPush))
	mux.HandleFunc("/api/logs", s.authRequired(s.handleLogs))
	mux.HandleFunc("/api/settings", s.authRequired(s.handleSettings))
	mux.HandleFunc("/api/push", s.authRequired(s.handlePush))
	mux.HandleFunc("/api/sources", s.authRequired(s.handleSources))
	mux.HandleFunc("/", s.handleSPA)
	return withRecover(s.log, mux)
}

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		golog.Child("[web]").Infof("控制台已启动: http://%s", displayAddr(addr))
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if s.static == nil {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	if _, err := fs.Stat(s.static, name); err != nil {
		name = "index.html"
	}
	serveFSFile(w, r, s.static, name)
}

func serveFSFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	f, err := fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if stat.IsDir() {
		serveFSFile(w, r, fsys, path.Join(name, "index.html"))
		return
	}
	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "static file not seekable", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), seeker)
}

func (s *Server) authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "未登录")
			return
		}
		user, ok := s.auth.Verify(token)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "登录已过期，请重新登录")
			return
		}
		next(w, r.WithContext(withUser(r.Context(), user)))
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	token, err := s.auth.Login(req.Username, req.Password, clientIP(r))
	if err != nil {
		s.log.Warnf("登录失败 user=%s ip=%s err=%v", req.Username, clientIP(r), err)
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	s.log.Infof("用户 %s 登录成功", req.Username)
	writeJSON(w, map[string]any{
		"token": token,
		"user":  s.auth.UserOf(req.Username),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.auth.UserOf(currentUser(r)))
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.auth.IsAdmin(currentUser(r)) {
		return true
	}
	writeErr(w, http.StatusForbidden, "当前账号为只读权限，无法执行该操作")
	return false
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.auth.ChangePassword(currentUser(r), req.OldPassword, req.NewPassword); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Infof("用户 %s 已修改密码", currentUser(r))
	writeJSON(w, map[string]string{"message": "密码已更新"})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.auth.ListUsers())
	case http.MethodPost:
		if !s.requireAdmin(w, r) {
			return
		}
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "请求格式错误")
			return
		}
		if err := s.auth.AddUser(req.Username, req.Password, req.Role); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		s.log.Infof("用户 %s 新增账号 %s", currentUser(r), req.Username)
		writeJSON(w, map[string]string{"message": "账号已创建"})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleUserItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	username := strings.TrimPrefix(r.URL.Path, "/api/users/")
	username = strings.Trim(username, "/")
	if err := s.auth.DeleteUser(currentUser(r), username); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Infof("用户 %s 删除账号 %s", currentUser(r), username)
	writeJSON(w, map[string]string{"message": "账号已删除"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.app.GetStats(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, stats)
}

func (s *Server) handleVulns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	query := ctrl.VulnQuery{
		Keyword:  q.Get("keyword"),
		Source:   q.Get("source"),
		Severity: q.Get("severity"),
		Page:     atoi(q.Get("page"), 1),
		Size:     atoi(q.Get("size"), 20),
	}
	if status := q.Get("status"); status == "pushed" {
		v := true
		query.Pushed = &v
	} else if status == "unpushed" {
		v := false
		query.Pushed = &v
	}
	items, total, err := s.app.ListVulns(r.Context(), query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"items": items, "total": total, "page": query.Page, "size": query.Size})
}

func (s *Server) handleVulnItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/vulns/")
	if rest == "" || rest == "push-unpushed" {
		s.handlePushUnpushed(w, r)
		return
	}
	parts := strings.Split(rest, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效的漏洞 ID")
		return
	}
	if len(parts) == 2 && parts[1] == "push" && r.Method == http.MethodPost {
		if !s.requireAdmin(w, r) {
			return
		}
		if err = s.app.PushVulnByID(r.Context(), id); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"message": "推送成功"})
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	item, err := s.app.GetVuln(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "漏洞不存在")
		return
	}
	writeJSON(w, item)
}

func (s *Server) handlePushUnpushed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	n, err := s.app.PushUnpushed(r.Context(), atoi(r.URL.Query().Get("limit"), 20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"pushed": n})
}

func (s *Server) handleDailyReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.app.SendDailyReport(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]string{"message": "每日报告已推送"})
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.app.TriggerCheck(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]string{"message": "已开始检查"})
}

func (s *Server) handleTestPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.app.TestPush(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]string{"message": "测试推送已发送"})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, total := s.logs.List(q.Get("level"), atoi(q.Get("page"), 1), atoi(q.Get("size"), 100))
	writeJSON(w, map[string]any{"items": items, "total": total})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.app.GetMonitorSettings())
	case http.MethodPut:
		if !s.requireAdmin(w, r) {
			return
		}
		var req ctrl.MonitorSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "请求格式错误")
			return
		}
		if err := s.app.ApplyMonitor(req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"message": "监控设置已保存"})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.app.GetPushSettings())
	case http.MethodPut:
		if !s.requireAdmin(w, r) {
			return
		}
		var req ctrl.PushSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "请求格式错误")
			return
		}
		if err := s.app.ApplyPush(req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"message": "推送设置已保存"})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, ctrl.KnownSources())
}

type ctxKey string

const userCtxKey ctxKey = "username"

func withUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, userCtxKey, username)
}

func currentUser(r *http.Request) string {
	v, _ := r.Context().Value(userCtxKey).(string)
	return v
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": status, "message": msg})
}

func atoi(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func withRecover(log *golog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Errorf("http panic: %v", rec)
				writeErr(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
