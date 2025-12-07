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

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	FailCount int       // 失败次数
	LastFail  time.Time // 最后失败时间
	LockedAt  time.Time // 锁定时间
}

// LoginRateLimiter 登录速率限制器
type LoginRateLimiter struct {
	mu       sync.RWMutex
	attempts map[string]*LoginAttempt // IP -> 尝试记录
}

// 速率限制配置
const (
	maxLoginAttempts  = 5               // 最大失败次数
	lockDuration      = 15 * time.Minute // 锁定时长
	attemptResetAfter = 30 * time.Minute // 失败计数重置时间
)

var loginLimiter = &LoginRateLimiter{
	attempts: make(map[string]*LoginAttempt),
}

// IsBlocked 检查 IP 是否被锁定
func (l *LoginRateLimiter) IsBlocked(ip string) (bool, time.Duration) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[ip]
	if !exists {
		return false, 0
	}

	// 检查是否在锁定期内
	if !attempt.LockedAt.IsZero() {
		remaining := lockDuration - time.Since(attempt.LockedAt)
		if remaining > 0 {
			return true, remaining
		}
	}

	return false, 0
}

// RecordFailure 记录登录失败
func (l *LoginRateLimiter) RecordFailure(ip string) (blocked bool, remaining time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.attempts[ip]
	if !exists {
		attempt = &LoginAttempt{}
		l.attempts[ip] = attempt
	}

	// 如果距离上次失败超过重置时间，重置计数
	if time.Since(attempt.LastFail) > attemptResetAfter {
		attempt.FailCount = 0
		attempt.LockedAt = time.Time{}
	}

	attempt.FailCount++
	attempt.LastFail = time.Now()

	// 达到最大失败次数，锁定账户
	if attempt.FailCount >= maxLoginAttempts {
		attempt.LockedAt = time.Now()
		return true, lockDuration
	}

	return false, 0
}

// RecordSuccess 记录登录成功（清除失败记录）
func (l *LoginRateLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// Cleanup 清理过期记录（可选，定期调用）
func (l *LoginRateLimiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, attempt := range l.attempts {
		// 清理超过1小时未活动的记录
		if now.Sub(attempt.LastFail) > time.Hour {
			delete(l.attempts, ip)
		}
	}
}

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
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand 不可用是严重错误，应立即终止
		panic("crypto/rand unavailable: " + err.Error())
	}
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

	// 获取客户端 IP
	clientIP := utils.GetClientIP(r)

	// 检查是否被速率限制锁定
	if blocked, remaining := loginLimiter.IsBlocked(clientIP); blocked {
		h.Audit(r, storage.AuditActionLoginFailed, "", "", false, map[string]string{
			"reason": "IP 被临时锁定",
			"ip":     clientIP,
		})
		utils.WriteErrorResponse(w, "TooManyRequests",
			"登录尝试次数过多，请 "+remaining.Round(time.Minute).String()+" 后重试",
			http.StatusTooManyRequests)
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
		// 记录失败并检查是否需要锁定
		blocked, remaining := loginLimiter.RecordFailure(clientIP)

		// 记录登录失败
		extra := map[string]string{
			"reason": "用户名或密码错误",
			"ip":     clientIP,
		}
		if blocked {
			extra["locked_for"] = remaining.String()
		}
		h.Audit(r, storage.AuditActionLoginFailed, req.Username, "", false, extra)

		if blocked {
			utils.WriteErrorResponse(w, "TooManyRequests",
				"登录尝试次数过多，账户已被临时锁定 "+remaining.Round(time.Minute).String(),
				http.StatusTooManyRequests)
		} else {
			utils.WriteErrorResponse(w, "Unauthorized", "用户名或密码错误", http.StatusUnauthorized)
		}
		return
	}

	// 登录成功，清除失败记录
	loginLimiter.RecordSuccess(clientIP)

	// 创建会话
	token := sessionStore.CreateSession()

	// 记录登录成功
	h.Audit(r, storage.AuditActionLogin, req.Username, "", true, nil)

	// 设置 cookie（根据请求协议设置 Secure 标志）
	isHTTPS := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS,
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
