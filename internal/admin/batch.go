package admin

import (
	"archive/zip"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sss/internal/utils"
)

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	Keys []string `json:"keys"` // 要删除的 key 列表
}

// BatchDeleteResult 批量删除结果
type BatchDeleteResult struct {
	DeletedCount int      `json:"deleted_count"` // 成功删除数量
	FailedCount  int      `json:"failed_count"`  // 失败数量
	FailedKeys   []string `json:"failed_keys"`   // 失败的 key 列表
}

// BatchDownloadRequest 批量下载请求
type BatchDownloadRequest struct {
	Keys []string `json:"keys"` // 要下载的 key 列表
}

// batchDeleteObjects 批量删除对象
func (h *Handler) batchDeleteObjects(w http.ResponseWriter, r *http.Request, bucketName string) {
	if r.Method != http.MethodPost {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	var req BatchDeleteRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		writeAdminError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	if len(req.Keys) == 0 {
		utils.WriteErrorResponse(w, "InvalidParameter", "keys is required", http.StatusBadRequest)
		return
	}

	// 限制单次批量删除数量
	if len(req.Keys) > 1000 {
		utils.WriteErrorResponse(w, "InvalidParameter", "Maximum 1000 keys per request", http.StatusBadRequest)
		return
	}

	result := BatchDeleteResult{
		FailedKeys: make([]string, 0),
	}

	for _, key := range req.Keys {
		// 安全检查：防止路径遍历
		if strings.Contains(key, "..") {
			result.FailedCount++
			result.FailedKeys = append(result.FailedKeys, key)
			continue
		}

		// 检查对象是否存在
		obj, err := h.metadata.GetObject(bucketName, key)
		if err != nil || obj == nil {
			result.FailedCount++
			result.FailedKeys = append(result.FailedKeys, key)
			continue
		}

		// 删除元数据
		if err := h.metadata.DeleteObject(bucketName, key); err != nil {
			result.FailedCount++
			result.FailedKeys = append(result.FailedKeys, key)
			continue
		}

		result.DeletedCount++
	}

	utils.WriteJSONResponse(w, result)
}

// batchDownloadObjects 批量下载对象（打包为 ZIP）
func (h *Handler) batchDownloadObjects(w http.ResponseWriter, r *http.Request, bucketName string) {
	if r.Method != http.MethodPost {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	var req BatchDownloadRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		writeAdminError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	if len(req.Keys) == 0 {
		utils.WriteErrorResponse(w, "InvalidParameter", "keys is required", http.StatusBadRequest)
		return
	}

	// 限制单次批量下载数量
	if len(req.Keys) > 100 {
		utils.WriteErrorResponse(w, "InvalidParameter", "Maximum 100 keys per request", http.StatusBadRequest)
		return
	}

	// Stage the archive before headers so missing or unreadable objects produce a
	// JSON error instead of a deceptively successful partial ZIP.
	archive, err := os.CreateTemp("", "sss-archive-*")
	if err != nil {
		writeAdminError(w, utils.ErrInternalError, 500, "")
		return
	}
	defer func() { archive.Close(); os.Remove(archive.Name()) }()
	zipWriter := zip.NewWriter(archive)
	for _, key := range req.Keys {
		obj, err := h.metadata.GetObject(bucketName, key)
		if err != nil || obj == nil {
			zipWriter.Close()
			writeAdminError(w, utils.ErrNoSuchKey, 404, "")
			return
		}
		reader, err := h.filestore.GetObject(obj.StoragePath)
		if err != nil {
			zipWriter.Close()
			writeAdminError(w, utils.ErrInternalError, 500, "")
			return
		}
		name := filepath.Base(key)
		if containsDuplicate(req.Keys, key) {
			name = key
		}
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
			reader.Close()
			zipWriter.Close()
			writeAdminError(w, utils.ErrInvalidArgument, 400, "")
			return
		}
		entry, err := zipWriter.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: obj.LastModified})
		if err == nil {
			_, err = io.Copy(entry, reader)
		}
		reader.Close()
		if err != nil {
			zipWriter.Close()
			writeAdminError(w, utils.ErrInternalError, 500, "")
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		writeAdminError(w, utils.ErrInternalError, 500, "")
		return
	}
	archive.Seek(0, io.SeekStart)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": bucketName + "-batch.zip"}))
	if _, err := io.Copy(w, archive); err != nil {
		utils.Error("archive response failed", "error", err)
	}
}

// containsDuplicate 检查是否有同名文件
func containsDuplicate(keys []string, currentKey string) bool {
	baseName := filepath.Base(currentKey)
	count := 0
	for _, k := range keys {
		if filepath.Base(k) == baseName {
			count++
		}
	}
	return count > 1
}
