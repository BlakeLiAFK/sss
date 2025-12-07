package admin

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"sss/internal/storage"
	"sss/internal/utils"
)

// AdminObjectInfo 管理员 API 对象信息
type AdminObjectInfo struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
	ETag         string `json:"etag"`
}

// adminObjectsHandler 对象操作处理器（支持 GET 列出和 DELETE 删除）
func (h *Handler) adminObjectsHandler(w http.ResponseWriter, r *http.Request, bucketName string) {
	switch r.Method {
	case http.MethodGet:
		h.adminListObjects(w, r, bucketName)
	case http.MethodDelete:
		h.adminDeleteObject(w, r, bucketName)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// adminListObjects 列出桶中的对象
func (h *Handler) adminListObjects(w http.ResponseWriter, r *http.Request, bucketName string) {
	prefix := r.URL.Query().Get("prefix")
	marker := r.URL.Query().Get("marker")
	maxKeys := 100

	result, err := h.metadata.ListObjects(bucketName, prefix, marker, "", maxKeys)
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

// adminDeleteObject 删除单个对象
// DELETE /api/admin/buckets/{bucket}/objects?key=xxx
func (h *Handler) adminDeleteObject(w http.ResponseWriter, r *http.Request, bucketName string) {
	key := r.URL.Query().Get("key")
	if key == "" {
		utils.WriteErrorResponse(w, "MissingParameter", "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	// 安全检查：防止路径遍历
	if strings.Contains(key, "..") {
		utils.WriteErrorResponse(w, "InvalidParameter", "Invalid key", http.StatusBadRequest)
		return
	}

	// 获取对象元数据
	obj, err := h.metadata.GetObject(bucketName, key)
	if err != nil {
		utils.Error("get object for delete failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	if obj == nil {
		utils.WriteError(w, utils.ErrNoSuchKey, http.StatusNotFound, "")
		return
	}

	// 删除文件
	if err := h.filestore.DeleteObject(obj.StoragePath); err != nil {
		utils.Error("delete file failed", "key", key, "error", err)
	}

	// 删除元数据
	if err := h.metadata.DeleteObject(bucketName, key); err != nil {
		utils.Error("delete metadata failed", "key", key, "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}

// adminUploadObject 上传对象
// POST /api/admin/buckets/{bucket}/upload?key=xxx
func (h *Handler) adminUploadObject(w http.ResponseWriter, r *http.Request, bucketName string) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		utils.WriteErrorResponse(w, "MissingParameter", "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	// 安全检查：防止路径遍历
	if strings.Contains(key, "..") {
		utils.WriteErrorResponse(w, "InvalidParameter", "Invalid key", http.StatusBadRequest)
		return
	}

	// 解析 multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.WriteErrorResponse(w, "MissingFile", "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 确定 content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 保存文件
	storagePath, etag, err := h.filestore.PutObject(bucketName, key, file, header.Size)
	if err != nil {
		utils.Error("save uploaded file failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 保存元数据
	obj := &storage.Object{
		Bucket:       bucketName,
		Key:          key,
		Size:         header.Size,
		ETag:         etag,
		ContentType:  contentType,
		StoragePath:  storagePath,
		LastModified: time.Now(),
	}
	if err := h.metadata.PutObject(obj); err != nil {
		utils.Error("save object metadata failed", "error", err)
		// 回滚：删除已上传的文件
		h.filestore.DeleteObject(storagePath)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"key":     key,
		"size":    header.Size,
		"etag":    etag,
	})
}

// adminDownloadObject 下载对象
// GET /api/admin/buckets/{bucket}/download?key=xxx
func (h *Handler) adminDownloadObject(w http.ResponseWriter, r *http.Request, bucketName string) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		utils.WriteErrorResponse(w, "MissingParameter", "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	// 安全检查：防止路径遍历
	if strings.Contains(key, "..") {
		utils.WriteErrorResponse(w, "InvalidParameter", "Invalid key", http.StatusBadRequest)
		return
	}

	// 获取对象元数据
	obj, err := h.metadata.GetObject(bucketName, key)
	if err != nil {
		utils.Error("get object for download failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	if obj == nil {
		utils.WriteError(w, utils.ErrNoSuchKey, http.StatusNotFound, "")
		return
	}

	// 读取文件
	file, err := h.filestore.GetObject(obj.StoragePath)
	if err != nil {
		utils.Error("read file for download failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	defer file.Close()

	// 设置响应头
	fileName := filepath.Base(key)
	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", obj.Size))
	w.Header().Set("ETag", obj.ETag)

	// 发送文件内容
	io.Copy(w, file)
}
