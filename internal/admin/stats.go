package admin

import (
	"net/http"
	"time"

	"sss/internal/storage"
	"sss/internal/utils"
)

// GCRequest 垃圾回收请求
type GCRequest struct {
	MaxUploadAge int  `json:"max_upload_age"` // 过期上传的最大年龄（小时）
	DryRun       bool `json:"dry_run"`        // 是否仅扫描不清理
}

// IntegrityRequest 完整性检查请求
type IntegrityRequest struct {
	VerifyEtag bool                     `json:"verify_etag"` // 是否验证 ETag
	Limit      int                      `json:"limit"`       // 检查数量限制
	Repair     bool                     `json:"repair"`      // 是否执行修复
	Issues     []storage.IntegrityIssue `json:"issues"`      // 需要修复的问题列表
}

// handleStorageStats 获取存储统计信息
func (h *Handler) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	stats, err := h.metadata.GetStorageStats()
	if err != nil {
		utils.Error("get storage stats failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	// 获取磁盘实际使用情况
	diskSize, fileCount, _ := h.filestore.GetDiskUsage()

	response := map[string]interface{}{
		"stats":           stats,
		"disk_usage":      diskSize,
		"disk_file_count": fileCount,
	}

	utils.WriteJSONResponse(w, response)
}

// handleRecentObjects 获取最近上传的对象
func (h *Handler) handleRecentObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	objects, err := h.metadata.GetRecentObjects(limit)
	if err != nil {
		utils.Error("get recent objects failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	result := make([]AdminObjectInfo, 0, len(objects))
	for _, obj := range objects {
		result = append(result, AdminObjectInfo{
			Key:          obj.Bucket + "/" + obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ETag:         obj.ETag,
		})
	}

	utils.WriteJSONResponse(w, result)
}

// parseInt 解析整数
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// handleGC 处理垃圾回收
// GET: 扫描并返回预览（dry run）
// POST: 执行清理
func (h *Handler) handleGC(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.scanGC(w, r)
	case http.MethodPost:
		h.executeGC(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// scanGC 扫描垃圾（预览模式）
func (h *Handler) scanGC(w http.ResponseWriter, r *http.Request) {
	// 默认过期时间 24 小时
	maxUploadAge := 24 * time.Hour

	// 从 query 参数获取自定义过期时间
	if ageStr := r.URL.Query().Get("max_upload_age"); ageStr != "" {
		if hours, err := parseInt(ageStr); err == nil && hours > 0 {
			maxUploadAge = time.Duration(hours) * time.Hour
		}
	}

	result, err := storage.RunGC(h.filestore, h.metadata, maxUploadAge, true)
	if err != nil {
		utils.Error("gc scan failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, result)
}

// executeGC 执行垃圾回收
func (h *Handler) executeGC(w http.ResponseWriter, r *http.Request) {
	var req GCRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		// 使用默认值
		req.MaxUploadAge = 24
		req.DryRun = false
	}

	// 默认过期时间 24 小时
	maxUploadAge := time.Duration(req.MaxUploadAge) * time.Hour
	if maxUploadAge <= 0 {
		maxUploadAge = 24 * time.Hour
	}

	result, err := storage.RunGC(h.filestore, h.metadata, maxUploadAge, req.DryRun)
	if err != nil {
		utils.Error("gc execute failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, result)
}

// handleIntegrity 处理完整性检查
// GET: 扫描并返回问题列表
// POST: 执行修复
func (h *Handler) handleIntegrity(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.checkIntegrity(w, r)
	case http.MethodPost:
		h.repairIntegrity(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// checkIntegrity 检查数据完整性
func (h *Handler) checkIntegrity(w http.ResponseWriter, r *http.Request) {
	// 是否验证 ETag（默认不验证，因为计算 MD5 较慢）
	verifyEtag := r.URL.Query().Get("verify_etag") == "true"

	// 检查数量限制（默认 1000）
	limit := 1000
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	result, err := storage.CheckIntegrity(h.filestore, h.metadata, verifyEtag, limit)
	if err != nil {
		utils.Error("integrity check failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, result)
}

// repairIntegrity 修复完整性问题
func (h *Handler) repairIntegrity(w http.ResponseWriter, r *http.Request) {
	var req IntegrityRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	// 如果没有提供问题列表，先扫描
	if len(req.Issues) == 0 {
		scanResult, err := storage.CheckIntegrity(h.filestore, h.metadata, req.VerifyEtag, req.Limit)
		if err != nil {
			utils.Error("integrity scan for repair failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
		req.Issues = scanResult.Issues
	}

	// 执行修复
	result, err := storage.RepairIntegrity(h.filestore, h.metadata, req.Issues)
	if err != nil {
		utils.Error("integrity repair failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, result)
}
