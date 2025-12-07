package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// SettingsResponse 系统设置响应
type SettingsResponse struct {
	Runtime RuntimeSettings `json:"runtime"` // 运行时参数（只读）
	Storage StorageSettings `json:"storage"` // 存储设置（可修改）
	System  SystemInfo      `json:"system"`  // 系统信息（只读）
}

// RuntimeSettings 运行时参数（启动时确定，不可在线修改）
type RuntimeSettings struct {
	Host     string `json:"host"`      // 监听地址
	Port     int    `json:"port"`      // 监听端口
	DataPath string `json:"data_path"` // 数据目录
	DBPath   string `json:"db_path"`   // 数据库路径
}

// StorageSettings 存储设置（可在线修改）
type StorageSettings struct {
	Region        string `json:"region"`          // S3 区域
	MaxObjectSize int64  `json:"max_object_size"` // 最大对象大小
	MaxUploadSize int64  `json:"max_upload_size"` // 最大上传大小
}

// SystemInfo 系统信息
type SystemInfo struct {
	Installed   bool   `json:"installed"`
	InstalledAt string `json:"installed_at"`
	Version     string `json:"version"`
}

// handleSettings 处理系统设置 API
func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSettings(w, r)
	case http.MethodPut:
		h.updateSettings(w, r)
	default:
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// getSettings 获取系统设置
func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	// 运行时参数（来自命令行，只读）
	runtime := RuntimeSettings{
		Host:     config.Global.Server.Host,
		Port:     config.Global.Server.Port,
		DataPath: config.Global.Storage.DataPath,
		DBPath:   config.Global.Storage.DBPath,
	}

	// 存储设置（可在线修改）
	storage_ := StorageSettings{
		Region:        config.Global.Server.Region,
		MaxObjectSize: config.Global.Storage.MaxObjectSize,
		MaxUploadSize: config.Global.Storage.MaxUploadSize,
	}

	// 系统信息
	installedAt, _ := h.metadata.GetSetting(storage.SettingSystemInstalledAt)
	version, _ := h.metadata.GetSetting(storage.SettingSystemVersion)

	resp := SettingsResponse{
		Runtime: runtime,
		Storage: storage_,
		System: SystemInfo{
			Installed:   h.metadata.IsInstalled(),
			InstalledAt: installedAt,
			Version:     version,
		},
	}

	utils.WriteJSONResponse(w, resp)
}

// UpdateSettingsRequest 更新设置请求（只包含可修改的字段）
type UpdateSettingsRequest struct {
	Region        *string `json:"region,omitempty"`
	MaxObjectSize *int64  `json:"max_object_size,omitempty"`
	MaxUploadSize *int64  `json:"max_upload_size,omitempty"`
}

// updateSettings 更新系统设置
func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 更新 S3 区域
	if req.Region != nil && *req.Region != "" {
		if err := h.metadata.SetSetting(storage.SettingServerRegion, *req.Region); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.Server.Region = *req.Region
	}

	// 更新最大对象大小
	if req.MaxObjectSize != nil && *req.MaxObjectSize > 0 {
		if err := h.metadata.SetSetting(storage.SettingStorageMaxObjectSize, strconv.FormatInt(*req.MaxObjectSize, 10)); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.Storage.MaxObjectSize = *req.MaxObjectSize
	}

	// 更新最大上传大小
	if req.MaxUploadSize != nil && *req.MaxUploadSize > 0 {
		if err := h.metadata.SetSetting(storage.SettingStorageMaxUploadSize, strconv.FormatInt(*req.MaxUploadSize, 10)); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.Storage.MaxUploadSize = *req.MaxUploadSize
	}

	// 记录审计日志
	h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "system", true, "更新系统设置")

	// 返回更新后的设置
	h.getSettings(w, r)
}

// handleChangePassword 修改管理员密码
func (h *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 验证必填字段
	req.OldPassword = strings.TrimSpace(req.OldPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.OldPassword == "" || req.NewPassword == "" {
		utils.WriteErrorResponse(w, "InvalidRequest", "密码不能为空", http.StatusBadRequest)
		return
	}

	// 验证新密码长度
	if len(req.NewPassword) < 6 {
		utils.WriteErrorResponse(w, "InvalidRequest", "新密码至少6个字符", http.StatusBadRequest)
		return
	}

	// 验证旧密码
	if !h.metadata.VerifyAdminPassword(req.OldPassword) {
		h.Audit(r, storage.AuditActionPasswordChange, "admin", "system", false, "旧密码验证失败")
		utils.WriteErrorResponse(w, "Unauthorized", "旧密码错误", http.StatusUnauthorized)
		return
	}

	// 设置新密码
	if err := h.metadata.SetAdminPassword(req.NewPassword); err != nil {
		utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录审计日志
	h.Audit(r, storage.AuditActionPasswordChange, "admin", "system", true, "管理员密码已更改")

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "密码修改成功",
	})
}
