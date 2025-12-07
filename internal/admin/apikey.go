package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sss/internal/auth"
	"sss/internal/storage"
	"sss/internal/utils"
)

// CreateAPIKeyRequest 创建 API Key 请求
type CreateAPIKeyRequest struct {
	Description string `json:"description"`
}

// APIKeyResponse API Key 响应
type APIKeyResponse struct {
	AccessKeyID     string                     `json:"access_key_id"`
	SecretAccessKey string                     `json:"secret_access_key,omitempty"`
	Description     string                     `json:"description"`
	CreatedAt       string                     `json:"created_at"`
	Enabled         bool                       `json:"enabled"`
	Permissions     []storage.APIKeyPermission `json:"permissions"`
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

// DeletePermissionRequest 删除权限请求
type DeletePermissionRequest struct {
	BucketName string `json:"bucket_name"`
}

// handleAPIKeys 处理 API Keys 列表/创建
func (h *Handler) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listAPIKeys(w, r)
	case http.MethodPost:
		h.createAPIKey(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// listAPIKeys 列出所有 API Keys
func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.metadata.ListAPIKeys()
	if err != nil {
		utils.Error("list api keys failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	result := make([]APIKeyResponse, 0, len(keys))
	for _, key := range keys {
		perms, _ := h.metadata.GetAPIKeyPermissions(key.AccessKeyID)
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
func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	key, err := h.metadata.CreateAPIKey(req.Description)
	if err != nil {
		utils.Error("create api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	// 记录审计日志
	h.Audit(r, storage.AuditActionAPIKeyCreate, "admin", key.AccessKeyID, true, map[string]string{
		"description": req.Description,
	})

	utils.WriteJSONResponse(w, APIKeyResponse{
		AccessKeyID:     key.AccessKeyID,
		SecretAccessKey: key.SecretAccessKey, // 只在创建时返回
		Description:     key.Description,
		CreatedAt:       key.CreatedAt.Format(time.RFC3339),
		Enabled:         key.Enabled,
		Permissions:     []storage.APIKeyPermission{},
	})
}

// handleAPIKeyDetail 处理单个 API Key 操作
func (h *Handler) handleAPIKeyDetail(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/", 2)
	accessKeyID := parts[0]

	// 检查 API Key 是否存在
	key, err := h.metadata.GetAPIKey(accessKeyID)
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
			h.getAPIKey(w, r, accessKeyID)
		case http.MethodPut:
			h.updateAPIKey(w, r, accessKeyID)
		case http.MethodDelete:
			h.deleteAPIKey(w, r, accessKeyID)
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
				h.setAPIKeyPermission(w, r, accessKeyID)
			case http.MethodDelete:
				h.deleteAPIKeyPermission(w, r, accessKeyID)
			default:
				utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
			}
		case "reset-secret":
			if r.Method == http.MethodPost {
				h.resetAPIKeySecret(w, r, accessKeyID)
			} else {
				utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
			}
		default:
			utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
		}
	}
}

// getAPIKey 获取 API Key 详情
func (h *Handler) getAPIKey(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	key, err := h.metadata.GetAPIKey(accessKeyID)
	if err != nil {
		utils.Error("get api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	perms, _ := h.metadata.GetAPIKeyPermissions(accessKeyID)

	utils.WriteJSONResponse(w, APIKeyResponse{
		AccessKeyID: key.AccessKeyID,
		Description: key.Description,
		CreatedAt:   key.CreatedAt.Format(time.RFC3339),
		Enabled:     key.Enabled,
		Permissions: perms,
	})
}

// updateAPIKey 更新 API Key
func (h *Handler) updateAPIKey(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	var req UpdateAPIKeyRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	if req.Description != nil {
		if err := h.metadata.UpdateAPIKeyDescription(accessKeyID, *req.Description); err != nil {
			utils.Error("update api key description failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
	}

	if req.Enabled != nil {
		if err := h.metadata.UpdateAPIKeyEnabled(accessKeyID, *req.Enabled); err != nil {
			utils.Error("update api key enabled failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	// 记录审计日志
	h.Audit(r, storage.AuditActionAPIKeyUpdate, "admin", accessKeyID, true, nil)

	h.getAPIKey(w, r, accessKeyID)
}

// deleteAPIKey 删除 API Key
func (h *Handler) deleteAPIKey(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	if err := h.metadata.DeleteAPIKey(accessKeyID); err != nil {
		utils.Error("delete api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	// 记录审计日志
	h.Audit(r, storage.AuditActionAPIKeyDelete, "admin", accessKeyID, true, nil)

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}

// setAPIKeyPermission 设置 API Key 权限
func (h *Handler) setAPIKeyPermission(w http.ResponseWriter, r *http.Request, accessKeyID string) {
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
		bucket, err := h.metadata.GetBucket(req.BucketName)
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

	if err := h.metadata.SetAPIKeyPermission(perm); err != nil {
		utils.Error("set api key permission failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	// 记录审计日志
	h.Audit(r, storage.AuditActionAPIKeySetPerm, "admin", accessKeyID, true, map[string]interface{}{
		"bucket":    req.BucketName,
		"can_read":  req.CanRead,
		"can_write": req.CanWrite,
	})

	h.getAPIKey(w, r, accessKeyID)
}

// deleteAPIKeyPermission 删除 API Key 权限
func (h *Handler) deleteAPIKeyPermission(w http.ResponseWriter, r *http.Request, accessKeyID string) {
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

	if err := h.metadata.DeleteAPIKeyPermission(accessKeyID, bucketName); err != nil {
		utils.Error("delete api key permission failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	// 记录审计日志
	h.Audit(r, storage.AuditActionAPIKeyDelPerm, "admin", accessKeyID, true, map[string]string{
		"bucket": bucketName,
	})

	h.getAPIKey(w, r, accessKeyID)
}

// resetAPIKeySecret 重置 API Key 的 Secret Key
func (h *Handler) resetAPIKeySecret(w http.ResponseWriter, r *http.Request, accessKeyID string) {
	newSecret, err := h.metadata.ResetAPIKeySecret(accessKeyID)
	if err != nil {
		utils.Error("reset api key secret failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 刷新缓存
	auth.ReloadAPIKeyCache()

	// 获取 API Key 详情
	key, err := h.metadata.GetAPIKey(accessKeyID)
	if err != nil {
		utils.Error("get api key failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	perms, _ := h.metadata.GetAPIKeyPermissions(accessKeyID)

	// 记录审计日志
	h.Audit(r, storage.AuditActionAPIKeyResetSecret, "admin", accessKeyID, true, nil)

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
