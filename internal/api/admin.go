package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"sss/internal/auth"
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

func generateSessionToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// checkAdminAuth 检查管理员认证
func (s *Server) checkAdminAuth(r *http.Request, w http.ResponseWriter) bool {
	token := r.Header.Get("X-Admin-Token")
	if token == "" {
		// 尝试从 cookie 获取
		if cookie, err := r.Cookie("admin_token"); err == nil {
			token = cookie.Value
		}
	}

	if token == "" || !sessionStore.ValidateSession(token) {
		utils.WriteErrorResponse(w, "Unauthorized", "Admin authentication required", http.StatusUnauthorized)
		return false
	}
	return true
}

// AdminLoginRequest 管理员登录请求
type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AdminLoginResponse 管理员登录响应
type AdminLoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// handleAdminLogin 处理管理员登录
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	var req AdminLoginRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	// 验证用户名和密码（使用常量时间比较防止时序攻击）
	usernameMatch := subtle.ConstantTimeCompare(
		[]byte(req.Username),
		[]byte(config.Global.Auth.AdminUsername),
	) == 1
	passwordMatch := subtle.ConstantTimeCompare(
		[]byte(req.Password),
		[]byte(config.Global.Auth.AdminPassword),
	) == 1

	if !usernameMatch || !passwordMatch {
		utils.WriteJSONResponse(w, AdminLoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// 创建会话
	token := sessionStore.CreateSession()

	// 设置 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(sessionDuration.Seconds()),
		SameSite: http.SameSiteStrictMode,
	})

	utils.WriteJSONResponse(w, AdminLoginResponse{
		Success: true,
		Token:   token,
	})
}

// handleAdminAPI 处理管理员 API
func (s *Server) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/")

	switch {
	case path == "logout":
		s.handleAdminLogout(w, r)
	case path == "apikeys":
		s.handleAPIKeys(w, r)
	case strings.HasPrefix(path, "apikeys/"):
		s.handleAPIKeyDetail(w, r, strings.TrimPrefix(path, "apikeys/"))
	case path == "buckets":
		s.handleAdminBucketsAPI(w, r)
	case strings.HasPrefix(path, "buckets/"):
		s.handleAdminBucketOps(w, r, strings.TrimPrefix(path, "buckets/"))
	default:
		utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
	}
}

// handleAdminLogout 处理管理员登出
func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
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

// AdminBucketInfo 管理员 API 桶信息
type AdminBucketInfo struct {
	Name         string `json:"name"`
	CreationDate string `json:"creation_date"`
	IsPublic     bool   `json:"is_public"`
}

// handleAdminBucketsAPI 管理员桶列表/创建 API
func (s *Server) handleAdminBucketsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.adminListBuckets(w, r)
	case http.MethodPost:
		s.adminCreateBucket(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// adminListBuckets 列出所有桶
func (s *Server) adminListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := s.metadata.ListBuckets()
	if err != nil {
		utils.Error("list buckets failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	result := make([]AdminBucketInfo, 0, len(buckets))
	for _, b := range buckets {
		result = append(result, AdminBucketInfo{
			Name:         b.Name,
			CreationDate: b.CreationDate.Format(time.RFC3339),
			IsPublic:     b.IsPublic,
		})
	}

	utils.WriteJSONResponse(w, result)
}

// CreateBucketRequest 创建桶请求
type CreateBucketRequest struct {
	Name string `json:"name"`
}

// adminCreateBucket 创建桶
func (s *Server) adminCreateBucket(w http.ResponseWriter, r *http.Request) {
	var req CreateBucketRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	if req.Name == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "Bucket name is required", http.StatusBadRequest)
		return
	}

	// 验证桶名
	if strings.Contains(req.Name, "..") || strings.ContainsAny(req.Name, "/\\") {
		utils.WriteErrorResponse(w, "InvalidBucketName", "Invalid bucket name", http.StatusBadRequest)
		return
	}

	// 检查桶是否已存在
	existing, err := s.metadata.GetBucket(req.Name)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	if existing != nil {
		utils.WriteErrorResponse(w, "BucketAlreadyExists", "Bucket already exists", http.StatusConflict)
		return
	}

	// 创建桶
	if err := s.metadata.CreateBucket(req.Name); err != nil {
		utils.Error("create bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 创建存储目录
	if err := s.filestore.CreateBucket(req.Name); err != nil {
		utils.Error("create bucket dir failed", "error", err)
		// 回滚数据库
		s.metadata.DeleteBucket(req.Name)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"name":    req.Name,
	})
}

// handleAdminBucketOps 管理员桶操作（删除、设置公开状态等）
func (s *Server) handleAdminBucketOps(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/", 2)
	bucketName := parts[0]

	if bucketName == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "Bucket name is required", http.StatusBadRequest)
		return
	}

	// 检查桶是否存在
	bucket, err := s.metadata.GetBucket(bucketName)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	if bucket == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "")
		return
	}

	if len(parts) == 1 {
		// /api/admin/buckets/{name} - 删除桶
		if r.Method == http.MethodDelete {
			s.adminDeleteBucket(w, r, bucketName)
		} else {
			utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		}
	} else {
		action := parts[1]
		switch action {
		case "public":
			s.adminSetBucketPublic(w, r, bucketName)
		case "objects":
			s.adminListObjects(w, r, bucketName)
		default:
			utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
		}
	}
}

// adminDeleteBucket 删除桶
func (s *Server) adminDeleteBucket(w http.ResponseWriter, r *http.Request, bucketName string) {
	if err := s.metadata.DeleteBucket(bucketName); err != nil {
		if strings.Contains(err.Error(), "not empty") {
			utils.WriteErrorResponse(w, "BucketNotEmpty", "Bucket is not empty", http.StatusConflict)
		} else {
			utils.Error("delete bucket failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		}
		return
	}

	// 删除存储目录
	s.filestore.DeleteBucket(bucketName)

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}

// SetBucketPublicRequest 设置桶公开状态请求
type SetBucketPublicRequest struct {
	IsPublic bool `json:"is_public"`
}

// adminSetBucketPublic 设置桶公开状态
func (s *Server) adminSetBucketPublic(w http.ResponseWriter, r *http.Request, bucketName string) {
	switch r.Method {
	case http.MethodGet:
		bucket, _ := s.metadata.GetBucket(bucketName)
		utils.WriteJSONResponse(w, map[string]bool{"is_public": bucket.IsPublic})
	case http.MethodPut:
		var req SetBucketPublicRequest
		if err := utils.ParseJSONBody(r, &req); err != nil {
			utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
			return
		}
		if err := s.metadata.UpdateBucketPublic(bucketName, req.IsPublic); err != nil {
			utils.Error("update bucket public failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
		utils.WriteJSONResponse(w, map[string]bool{"is_public": req.IsPublic})
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// AdminObjectInfo 管理员 API 对象信息
type AdminObjectInfo struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
	ETag         string `json:"etag"`
}

// adminListObjects 列出桶中的对象
func (s *Server) adminListObjects(w http.ResponseWriter, r *http.Request, bucketName string) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	prefix := r.URL.Query().Get("prefix")
	marker := r.URL.Query().Get("marker")
	maxKeys := 100

	result, err := s.metadata.ListObjects(bucketName, prefix, marker, "", maxKeys)
	if err != nil {
		utils.Error("list objects failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	objects := make([]AdminObjectInfo, 0, len(result.Contents))
	for _, obj := range result.Contents {
		objects = append(objects, AdminObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ETag:         obj.ETag,
		})
	}

	utils.WriteJSONResponse(w, map[string]interface{}{
		"objects":      objects,
		"is_truncated": result.IsTruncated,
		"next_marker":  result.NextMarker,
	})
}

// CreateAPIKeyRequest 创建 API Key 请求
type CreateAPIKeyRequest struct {
	Description string `json:"description"`
}

// APIKeyResponse API Key 响应
type APIKeyResponse struct {
	AccessKeyID     string                    `json:"access_key_id"`
	SecretAccessKey string                    `json:"secret_access_key,omitempty"`
	Description     string                    `json:"description"`
	CreatedAt       string                    `json:"created_at"`
	Enabled         bool                      `json:"enabled"`
	Permissions     []storage.APIKeyPermission `json:"permissions"`
}

// handleAPIKeys 处理 API Keys 列表/创建
func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listAPIKeys(w, r)
	case http.MethodPost:
		s.createAPIKey(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// listAPIKeys 列出所有 API Keys
func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.metadata.ListAPIKeys()
	if err != nil {
		utils.Error("list api keys failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	result := make([]APIKeyResponse, 0, len(keys))
	for _, key := range keys {
		perms, _ := s.metadata.GetAPIKeyPermissions(key.AccessKeyID)
		result = append(result, APIKeyResponse{
			AccessKeyID: key.AccessKeyID,
			Description: key.Description,
			CreatedAt:   key.CreatedAt.Format(time.RFC3339),
			Enabled:     key.Enabled,
			Permissions: perms,
		})
	}

	utils.WriteJSONResponse(w, result)
}

// createAPIKey 创建 API Key
func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	key, err := s.metadata.CreateAPIKey(req.Description)
	if err != nil {
		utils.Error("create api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	utils.WriteJSONResponse(w, APIKeyResponse{
		AccessKeyID:     key.AccessKeyID,
		SecretAccessKey: key.SecretAccessKey, // 只在创建时返回
		Description:     key.Description,
		CreatedAt:       key.CreatedAt.Format(time.RFC3339),
		Enabled:         key.Enabled,
		Permissions:     []storage.APIKeyPermission{},
	})
}

// UpdateAPIKeyRequest 更新 API Key 请求
type UpdateAPIKeyRequest struct {
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// SetPermissionRequest 设置权限请求
type SetPermissionRequest struct {
	BucketName string `json:"bucket_name"`
	CanRead    bool   `json:"can_read"`
	CanWrite   bool   `json:"can_write"`
}

// handleAPIKeyDetail 处理单个 API Key 操作
func (s *Server) handleAPIKeyDetail(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/", 2)
	accessKeyID := parts[0]

	// 检查 API Key 是否存在
	key, err := s.metadata.GetAPIKey(accessKeyID)
	if err != nil {
		utils.Error("get api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	if key == nil {
		utils.WriteErrorResponse(w, "NotFound", "API Key not found", http.StatusNotFound)
		return
	}

	// 路由到具体操作
	if len(parts) == 1 {
		// /api/admin/apikeys/{id}
		switch r.Method {
		case http.MethodGet:
			s.getAPIKey(w, r, accessKeyID)
		case http.MethodPut:
			s.updateAPIKey(w, r, accessKeyID)
		case http.MethodDelete:
			s.deleteAPIKey(w, r, accessKeyID)
		default:
			utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		}
	} else {
		// /api/admin/apikeys/{id}/permissions 或 /reset-secret
		action := parts[1]
		switch action {
		case "permissions":
			switch r.Method {
			case http.MethodPost:
				s.setAPIKeyPermission(w, r, accessKeyID)
			case http.MethodDelete:
				s.deleteAPIKeyPermission(w, r, accessKeyID)
			default:
				utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
			}
		case "reset-secret":
			if r.Method == http.MethodPost {
				s.resetAPIKeySecret(w, r, accessKeyID)
			} else {
				utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
			}
		default:
			utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
		}
	}
}

// getAPIKey 获取 API Key 详情
func (s *Server) getAPIKey(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	key, err := s.metadata.GetAPIKey(accessKeyID)
	if err != nil {
		utils.Error("get api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	perms, _ := s.metadata.GetAPIKeyPermissions(accessKeyID)

	utils.WriteJSONResponse(w, APIKeyResponse{
		AccessKeyID: key.AccessKeyID,
		Description: key.Description,
		CreatedAt:   key.CreatedAt.Format(time.RFC3339),
		Enabled:     key.Enabled,
		Permissions: perms,
	})
}

// updateAPIKey 更新 API Key
func (s *Server) updateAPIKey(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	var req UpdateAPIKeyRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	if req.Description != nil {
		if err := s.metadata.UpdateAPIKeyDescription(accessKeyID, *req.Description); err != nil {
			utils.Error("update api key description failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
	}

	if req.Enabled != nil {
		if err := s.metadata.UpdateAPIKeyEnabled(accessKeyID, *req.Enabled); err != nil {
			utils.Error("update api key enabled failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	s.getAPIKey(w, r, accessKeyID)
}

// deleteAPIKey 删除 API Key
func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	if err := s.metadata.DeleteAPIKey(accessKeyID); err != nil {
		utils.Error("delete api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}

// setAPIKeyPermission 设置 API Key 权限
func (s *Server) setAPIKeyPermission(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	var req SetPermissionRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	if req.BucketName == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "bucket_name is required", http.StatusBadRequest)
		return
	}

	// 验证桶名（如果不是通配符）
	if req.BucketName != "*" {
		bucket, err := s.metadata.GetBucket(req.BucketName)
		if err != nil {
			utils.Error("check bucket failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
		if bucket == nil {
			utils.WriteErrorResponse(w, "InvalidParameter", "Bucket does not exist", http.StatusBadRequest)
			return
		}
	}

	perm := &storage.APIKeyPermission{
		AccessKeyID: accessKeyID,
		BucketName:  req.BucketName,
		CanRead:     req.CanRead,
		CanWrite:    req.CanWrite,
	}

	if err := s.metadata.SetAPIKeyPermission(perm); err != nil {
		utils.Error("set api key permission failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	s.getAPIKey(w, r, accessKeyID)
}

// DeletePermissionRequest 删除权限请求
type DeletePermissionRequest struct {
	BucketName string `json:"bucket_name"`
}

// deleteAPIKeyPermission 删除 API Key 权限
func (s *Server) deleteAPIKeyPermission(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	// 从 query 参数获取 bucket_name
	bucketName := r.URL.Query().Get("bucket_name")
	if bucketName == "" {
		// 尝试从 body 获取
		var req DeletePermissionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			bucketName = req.BucketName
		}
	}

	if bucketName == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "bucket_name is required", http.StatusBadRequest)
		return
	}

	if err := s.metadata.DeleteAPIKeyPermission(accessKeyID, bucketName); err != nil {
		utils.Error("delete api key permission failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	s.getAPIKey(w, r, accessKeyID)
}

// resetAPIKeySecret 重置 API Key 的 Secret Key
func (s *Server) resetAPIKeySecret(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	newSecret, err := s.metadata.ResetAPIKeySecret(accessKeyID)
	if err != nil {
		utils.Error("reset api key secret failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	// 获取 API Key 详情
	key, err := s.metadata.GetAPIKey(accessKeyID)
	if err != nil {
		utils.Error("get api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	perms, _ := s.metadata.GetAPIKeyPermissions(accessKeyID)

	// 返回包含新 Secret Key 的响应（仅此次返回）
	utils.WriteJSONResponse(w, APIKeyResponse{
		AccessKeyID:     key.AccessKeyID,
		SecretAccessKey: newSecret,
		Description:     key.Description,
		CreatedAt:       key.CreatedAt.Format(time.RFC3339),
		Enabled:         key.Enabled,
		Permissions:     perms,
	})
}
