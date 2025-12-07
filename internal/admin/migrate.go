package admin

import (
	"net/http"
	"strings"

	"sss/internal/storage"
	"sss/internal/utils"
)

// MigrateRequest 迁移请求
type MigrateRequest struct {
	SourceEndpoint  string `json:"sourceEndpoint"`
	SourceAccessKey string `json:"sourceAccessKey"`
	SourceSecretKey string `json:"sourceSecretKey"`
	SourceBucket    string `json:"sourceBucket"`
	SourcePrefix    string `json:"sourcePrefix"`
	SourceRegion    string `json:"sourceRegion"`
	TargetBucket    string `json:"targetBucket"`
	TargetPrefix    string `json:"targetPrefix"`
	OverwriteExist  bool   `json:"overwriteExist"`
}

// handleMigrateAPI 处理迁移 API
// GET: 获取所有任务列表
// POST: 创建新迁移任务
func (h *Handler) handleMigrateAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listMigrateJobs(w, r)
	case http.MethodPost:
		h.createMigrateJob(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// listMigrateJobs 获取所有迁移任务
func (h *Handler) listMigrateJobs(w http.ResponseWriter, r *http.Request) {
	mgr := storage.GetMigrateManager(h.metadata, h.filestore)
	jobs := mgr.GetAllJobs()

	// 获取统计信息
	stats := mgr.GetJobStats()

	utils.WriteJSONResponse(w, map[string]interface{}{
		"jobs":  jobs,
		"stats": stats,
	})
}

// createMigrateJob 创建迁移任务
func (h *Handler) createMigrateJob(w http.ResponseWriter, r *http.Request) {
	var req MigrateRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	// 转换为 MigrateConfig
	cfg := storage.MigrateConfig{
		SourceEndpoint:  req.SourceEndpoint,
		SourceAccessKey: req.SourceAccessKey,
		SourceSecretKey: req.SourceSecretKey,
		SourceBucket:    req.SourceBucket,
		SourcePrefix:    req.SourcePrefix,
		SourceRegion:    req.SourceRegion,
		TargetBucket:    req.TargetBucket,
		TargetPrefix:    req.TargetPrefix,
		OverwriteExist:  req.OverwriteExist,
	}

	mgr := storage.GetMigrateManager(h.metadata, h.filestore)
	jobID, err := mgr.StartMigration(cfg)
	if err != nil {
		utils.WriteErrorResponse(w, "MigrationError", err.Error(), http.StatusBadRequest)
		return
	}

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"jobId":   jobID,
	})
}

// handleMigrateJob 处理单个迁移任务操作
// GET /api/admin/migrate/{jobId}: 获取任务进度
// DELETE /api/admin/migrate/{jobId}: 取消任务
// POST /api/admin/migrate/validate: 验证连接配置
func (h *Handler) handleMigrateJob(w http.ResponseWriter, r *http.Request, path string) {
	// 特殊处理 validate 端点
	if path == "validate" {
		if r.Method == http.MethodPost {
			h.validateMigrateConfig(w, r)
		} else {
			utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		}
		return
	}

	// 处理任务 ID
	parts := strings.SplitN(path, "/", 2)
	jobID := parts[0]

	if jobID == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "Job ID is required", http.StatusBadRequest)
		return
	}

	mgr := storage.GetMigrateManager(h.metadata, h.filestore)

	// 检查任务是否存在
	progress := mgr.GetProgress(jobID)
	if progress == nil {
		utils.WriteErrorResponse(w, "NotFound", "Job not found", http.StatusNotFound)
		return
	}

	// 处理子路由
	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "cancel":
			if r.Method == http.MethodPost {
				h.cancelMigrateJob(w, r, jobID)
			} else {
				utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
			}
		default:
			utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
		}
		return
	}

	// 处理主路由
	switch r.Method {
	case http.MethodGet:
		utils.WriteJSONResponse(w, progress)
	case http.MethodDelete:
		h.deleteMigrateJob(w, r, jobID)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// validateMigrateConfig 验证迁移配置（连接测试）
func (h *Handler) validateMigrateConfig(w http.ResponseWriter, r *http.Request) {
	var req MigrateRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	cfg := storage.MigrateConfig{
		SourceEndpoint:  req.SourceEndpoint,
		SourceAccessKey: req.SourceAccessKey,
		SourceSecretKey: req.SourceSecretKey,
		SourceBucket:    req.SourceBucket,
		SourceRegion:    req.SourceRegion,
	}

	mgr := storage.GetMigrateManager(h.metadata, h.filestore)
	err := mgr.ValidateMigrateConfig(cfg)
	if err != nil {
		utils.WriteJSONResponse(w, map[string]interface{}{
			"valid":   false,
			"message": err.Error(),
		})
		return
	}

	utils.WriteJSONResponse(w, map[string]interface{}{
		"valid":   true,
		"message": "Connection successful",
	})
}

// cancelMigrateJob 取消迁移任务
func (h *Handler) cancelMigrateJob(w http.ResponseWriter, r *http.Request, jobID string) {
	mgr := storage.GetMigrateManager(h.metadata, h.filestore)
	err := mgr.CancelMigration(jobID)
	if err != nil {
		utils.WriteErrorResponse(w, "CancelError", err.Error(), http.StatusBadRequest)
		return
	}

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}

// deleteMigrateJob 删除迁移任务记录
func (h *Handler) deleteMigrateJob(w http.ResponseWriter, r *http.Request, jobID string) {
	mgr := storage.GetMigrateManager(h.metadata, h.filestore)
	err := mgr.DeleteJob(jobID)
	if err != nil {
		utils.WriteErrorResponse(w, "DeleteError", err.Error(), http.StatusBadRequest)
		return
	}

	utils.WriteJSONResponse(w, map[string]bool{"success": true})
}
