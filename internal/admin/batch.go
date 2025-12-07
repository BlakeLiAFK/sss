package admin

import (
	"archive/zip"
	"io"
	"net/http"
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
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	var req BatchDeleteRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
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

		// 删除文件
		if err := h.filestore.DeleteObject(obj.StoragePath); err != nil {
			utils.Error("batch delete file failed", "key", key, "error", err)
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
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	var req BatchDownloadRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
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

	// 设置响应头
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+bucketName+"-batch.zip\"")

	// 创建 ZIP 写入器
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, key := range req.Keys {
		// 安全检查：防止路径遍历
		if strings.Contains(key, "..") {
			continue
		}

		// 获取对象元数据
		obj, err := h.metadata.GetObject(bucketName, key)
		if err != nil || obj == nil {
			continue
		}

		// 创建 ZIP 条目
		header := &zip.FileHeader{
			Name:     filepath.Base(key), // 使用文件名而非完整路径
			Method:   zip.Deflate,
			Modified: obj.LastModified,
		}

		// 如果有同名文件，使用完整路径
		if containsDuplicate(req.Keys, key) {
			header.Name = key
		}

		zipEntry, err := zipWriter.CreateHeader(header)
		if err != nil {
			utils.Error("create zip entry failed", "key", key, "error", err)
			continue
		}

		// 读取并写入文件内容
		reader, err := h.filestore.GetObject(obj.StoragePath)
		if err != nil {
			utils.Error("read file for zip failed", "key", key, "error", err)
			continue
		}

		_, err = io.Copy(zipEntry, reader)
		reader.Close()
		if err != nil {
			utils.Error("write to zip failed", "key", key, "error", err)
		}
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
