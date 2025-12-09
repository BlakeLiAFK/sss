package admin

import (
	"net/http"
	"strings"
	"time"

	"sss/internal/storage"
	"sss/internal/utils"
)

// AdminBucketInfo 管理员 API 桶信息
type AdminBucketInfo struct {
	Name         string `json:"name"`
	CreationDate string `json:"creation_date"`
	IsPublic     bool   `json:"is_public"`
}

// CreateBucketRequest 创建桶请求
type CreateBucketRequest struct {
	Name string `json:"name"`
}

// SetBucketPublicRequest 设置桶公开状态请求
type SetBucketPublicRequest struct {
	IsPublic bool `json:"is_public"`
}

// handleAdminBucketsAPI 管理员桶列表/创建 API
func (h *Handler) handleAdminBucketsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.adminListBuckets(w, r)
	case http.MethodPost:
		h.adminCreateBucket(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// adminListBuckets 列出所有桶
func (h *Handler) adminListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.metadata.ListBuckets()
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

// adminCreateBucket 创建桶
func (h *Handler) adminCreateBucket(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.metadata.GetBucket(req.Name)
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
	if err := h.metadata.CreateBucket(req.Name); err != nil {
		utils.Error("create bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 创建存储目录
	if err := h.filestore.CreateBucket(req.Name); err != nil {
		utils.Error("create bucket dir failed", "error", err)
		// 回滚数据库
		h.metadata.DeleteBucket(req.Name)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 记录审计日志
	h.Audit(r, storage.AuditActionBucketCreate, "admin", req.Name, true, nil)

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"name":    req.Name,
	})
}

// handleAdminBucketOps 管理员桶操作（删除、设置公开状态等）
func (h *Handler) handleAdminBucketOps(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/", 2)
	bucketName := parts[0]

	if bucketName == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "Bucket name is required", http.StatusBadRequest)
		return
	}

	// 检查桶是否存在
	bucket, err := h.metadata.GetBucket(bucketName)
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
		// /api/admin/buckets/{name} - 桶操作
		switch r.Method {
		case http.MethodGet:
			// 获取桶详情
			utils.WriteJSONResponse(w, AdminBucketInfo{
				Name:         bucket.Name,
				CreationDate: bucket.CreationDate.Format(time.RFC3339),
				IsPublic:     bucket.IsPublic,
			})
		case http.MethodPut:
			// 更新桶设置（公开状态）
			var req struct {
				IsPublic bool `json:"isPublic"`
			}
			if err := utils.ParseJSONBody(r, &req); err != nil {
				utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
				return
			}
			if err := h.metadata.UpdateBucketPublic(bucketName, req.IsPublic); err != nil {
				utils.Error("update bucket public failed", "error", err)
				utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
				return
			}
			utils.WriteJSONResponse(w, map[string]interface{}{
				"success":  true,
				"isPublic": req.IsPublic,
			})
		case http.MethodDelete:
			h.adminDeleteBucket(w, r, bucketName)
		default:
			utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		}
	} else {
		action := parts[1]
		switch action {
		case "public":
			h.adminSetBucketPublic(w, r, bucketName)
		case "objects":
			h.adminObjectsHandler(w, r, bucketName)
		case "upload":
			h.adminUploadObject(w, r, bucketName)
		case "download":
			h.adminDownloadObject(w, r, bucketName)
		case "copy":
			h.adminCopyObject(w, r, bucketName)
		case "search":
			h.adminSearchObjects(w, r, bucketName)
		case "batch/delete":
			h.batchDeleteObjects(w, r, bucketName)
		case "batch/download":
			h.batchDownloadObjects(w, r, bucketName)
		case "preview":
			h.previewObject(w, r, bucketName)
		default:
			utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
		}
	}
}

// adminDeleteBucket 删除桶
func (h *Handler) adminDeleteBucket(w http.ResponseWriter, r *http.Request, bucketName string) {
	if err := h.metadata.DeleteBucket(bucketName); err != nil {
		if strings.Contains(err.Error(), "not empty") {
			utils.WriteErrorResponse(w, "BucketNotEmpty", "Bucket is not empty", http.StatusConflict)
		} else {
			utils.Error("delete bucket failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		}
		return
	}

	// 删除存储目录
	h.filestore.DeleteBucket(bucketName)

	// 记录审计日志
	h.Audit(r, storage.AuditActionBucketDelete, "admin", bucketName, true, nil)

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}

// adminSetBucketPublic 设置桶公开状态
func (h *Handler) adminSetBucketPublic(w http.ResponseWriter, r *http.Request, bucketName string) {
	switch r.Method {
	case http.MethodGet:
		bucket, _ := h.metadata.GetBucket(bucketName)
		utils.WriteJSONResponse(w, map[string]bool{"is_public": bucket.IsPublic})
	case http.MethodPut:
		var req SetBucketPublicRequest
		if err := utils.ParseJSONBody(r, &req); err != nil {
			utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
			return
		}
		if err := h.metadata.UpdateBucketPublic(bucketName, req.IsPublic); err != nil {
			utils.Error("update bucket public failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
		// 记录审计日志
		if req.IsPublic {
			h.Audit(r, storage.AuditActionBucketSetPublic, "admin", bucketName, true, nil)
		} else {
			h.Audit(r, storage.AuditActionBucketSetPrivate, "admin", bucketName, true, nil)
		}
		utils.WriteJSONResponse(w, map[string]bool{"is_public": req.IsPublic})
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}
