package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// Session 管理员会话
type Session struct {
	Token     string
	ExpiresAt time.Time
}

// SessionStore 会话存储
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

var sessionStore = &SessionStore{
	sessions: make(map[string]*Session),
}

// 会话有效期 24 小时
const sessionDuration = 24 * time.Hour

// CreateSession 创建会话
func (s *SessionStore) CreateSession() string {
	token := generateSessionToken()
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清理过期会话
	now := time.Now()
	for k, v := range s.sessions {
		if now.After(v.ExpiresAt) {
			delete(s.sessions, k)
		}
	}

	s.sessions[token] = &Session{
		Token:     token,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	return token
}

// ValidateSession 验证会话
func (s *SessionStore) ValidateSession(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[token]
	if !exists {
		return false
	}
	return time.Now().Before(session.ExpiresAt)
}

// DeleteSession 删除会话
func (s *SessionStore) DeleteSession(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// generateSessionToken 生成会话令牌
func generateSessionToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// checkAdminAuth 检查管理员认证
// checkAdminAuth 检查管理员认证
func (h *Handler) checkAdminAuth(r *http.Request) bool {
	token := r.Header.Get("X-Admin-Token")
	if token == "" {
		// 尝试从 cookie 获取
		if cookie, err := r.Cookie("admin_token"); err == nil {
			token = cookie.Value
		}
	}
	return token != "" && sessionStore.ValidateSession(token)
}

// AdminLoginRequest 管理员登录请求
type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AdminLoginResponse 管理员登录响应
type AdminLoginResponse struct {
	Success         bool   `json:"success"`
	Token           string `json:"token,omitempty"`
	Message         string `json:"message,omitempty"`
	AccessKeyId     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
}

// handleAdminLogin 处理管理员登录
func (h *Handler) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	// 检查系统是否已安装
	if !h.metadata.IsInstalled() {
		utils.WriteErrorResponse(w, "NotInstalled", "系统尚未安装，请先完成安装", http.StatusForbidden)
		return
	}

	var req AdminLoginRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	// 验证用户名
	usernameMatch := subtle.ConstantTimeCompare(
		[]byte(req.Username),
		[]byte(h.metadata.GetAdminUsername()),
	) == 1

	// 验证密码（所有密码都存储在数据库中，使用 bcrypt 验证）
	passwordMatch := h.metadata.VerifyAdminPassword(req.Password)

	if !usernameMatch || !passwordMatch {
		// 记录登录失败
		h.Audit(r, storage.AuditActionLoginFailed, req.Username, "", false, map[string]string{
			"reason": "用户名或密码错误",
		})
		utils.WriteErrorResponse(w, "Unauthorized", "用户名或密码错误", http.StatusUnauthorized)
		return
	}

	// 创建会话
	token := sessionStore.CreateSession()

	// 记录登录成功
	h.Audit(r, storage.AuditActionLogin, req.Username, "", true, nil)

	// 设置 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(sessionDuration.Seconds()),
		SameSite: http.SameSiteStrictMode,
	})

	// 获取 API Key
	accessKeyID, secretAccessKey := h.metadata.GetAuthConfig()
	if accessKeyID == "" {
		// 兼容旧配置
		accessKeyID = config.Global.Auth.AccessKeyID
		secretAccessKey = config.Global.Auth.SecretAccessKey
	}

	utils.WriteJSONResponse(w, AdminLoginResponse{
		Success:         true,
		Token:           token,
		AccessKeyId:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	})
}

// handleAdminLogout 处理管理员登出
func (h *Handler) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	token := r.Header.Get("X-Admin-Token")
	if token == "" {
		if cookie, err := r.Cookie("admin_token"); err == nil {
			token = cookie.Value
		}
	}

	if token != "" {
		sessionStore.DeleteSession(token)
	}

	// 记录登出
	h.Audit(r, storage.AuditActionLogout, "admin", "", true, nil)

	// 清除 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}
