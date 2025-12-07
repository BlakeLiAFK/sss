package admin

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sss/internal/auth"
	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// 密码重置文件路径
const resetPasswordFile = "./data/.reset_password"

// SetupRequest 安装请求
type SetupRequest struct {
	AdminUsername   string `json:"admin_username"`
	AdminPassword   string `json:"admin_password"`
	ServerHost      string `json:"server_host"`
	ServerPort      string `json:"server_port"`
	ServerRegion    string `json:"server_region"`
	StorageDataPath string `json:"storage_data_path"`
}

// SetupResponse 安装响应
type SetupResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
}

// SystemStatusResponse 系统状态响应
type SystemStatusResponse struct {
	Installed   bool   `json:"installed"`
	InstalledAt string `json:"installed_at,omitempty"`
	Version     string `json:"version"`
}

// ResetPasswordCheckResponse 密码重置检测响应
type ResetPasswordCheckResponse struct {
	FileExists bool   `json:"file_exists"`
	FilePath   string `json:"file_path"`
	Command    string `json:"command"`
}

// ResetPasswordRequest 密码重置请求
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// handleSetupAPI 处理安装相关 API
func (h *Handler) handleSetupAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/setup")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" || path == "status":
		h.handleSystemStatus(w, r)
	case path == "install":
		h.handleInstall(w, r)
	case path == "reset-password/check":
		h.handleResetPasswordCheck(w, r)
	case path == "reset-password":
		h.handleResetPassword(w, r)
	default:
		utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
	}
}

// handleSystemStatus 获取系统状态
func (h *Handler) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	installed := h.metadata.IsInstalled()
	var installedAt string
	if installed {
		installedAt, _ = h.metadata.GetSetting(storage.SettingSystemInstalledAt)
	}

	version, _ := h.metadata.GetSetting(storage.SettingSystemVersion)
	if version == "" {
		version = "1.1.0"
	}

	utils.WriteJSONResponse(w, SystemStatusResponse{
		Installed:   installed,
		InstalledAt: installedAt,
		Version:     version,
	})
}

// handleInstall 处理安装请求
func (h *Handler) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	// 检查是否已安装
	if h.metadata.IsInstalled() {
		utils.WriteErrorResponse(w, "AlreadyInstalled", "系统已安装，无法重复安装", http.StatusBadRequest)
		return
	}

	var req SetupRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	// 验证必填参数
	if req.AdminUsername == "" {
		req.AdminUsername = "admin"
	}
	if req.AdminPassword == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "管理员密码不能为空", http.StatusBadRequest)
		return
	}
	// 使用统一的密码复杂度验证
	if err := storage.ValidatePassword(req.AdminPassword); err != nil {
		utils.WriteErrorResponse(w, "InvalidParameter", err.Error(), http.StatusBadRequest)
		return
	}

	// 设置默认值
	if req.ServerHost == "" {
		req.ServerHost = "0.0.0.0"
	}
	if req.ServerPort == "" {
		req.ServerPort = "8080"
	}
	if req.ServerRegion == "" {
		req.ServerRegion = "us-east-1"
	}
	if req.StorageDataPath == "" {
		req.StorageDataPath = "./data/buckets"
	}

	// 初始化配置
	if err := h.metadata.InitDefaultSettings(req.AdminUsername, req.AdminPassword); err != nil {
		utils.Error("初始化配置失败", "error", err)
		utils.WriteErrorResponse(w, "InternalError", "初始化配置失败", http.StatusInternalServerError)
		return
	}

	// 设置自定义配置
	h.metadata.SetSetting(storage.SettingServerHost, req.ServerHost)
	h.metadata.SetSetting(storage.SettingServerPort, req.ServerPort)
	h.metadata.SetSetting(storage.SettingServerRegion, req.ServerRegion)
	h.metadata.SetSetting(storage.SettingStorageDataPath, req.StorageDataPath)

	// 标记为已安装
	if err := h.metadata.SetInstalled(); err != nil {
		utils.Error("设置安装状态失败", "error", err)
		utils.WriteErrorResponse(w, "InternalError", "设置安装状态失败", http.StatusInternalServerError)
		return
	}

	// 获取生成的 API Key
	accessKeyID, secretAccessKey := h.metadata.GetAuthConfig()

	// 重新加载配置到全局
	ReloadConfigFromDB(h.metadata)

	// 记录安装审计日志
	h.Audit(r, storage.AuditActionSystemInstall, req.AdminUsername, "", true, map[string]string{
		"server_host": req.ServerHost,
		"server_port": req.ServerPort,
	})

	utils.WriteJSONResponse(w, SetupResponse{
		Success:         true,
		Message:         "安装成功",
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	})
}

// handleResetPasswordCheck 检测密码重置文件是否存在
func (h *Handler) handleResetPasswordCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	// 获取绝对路径
	absPath, _ := filepath.Abs(resetPasswordFile)

	// 检测文件是否存在
	_, err := os.Stat(resetPasswordFile)
	fileExists := err == nil

	utils.WriteJSONResponse(w, ResetPasswordCheckResponse{
		FileExists: fileExists,
		FilePath:   absPath,
		Command:    "touch " + absPath,
	})
}

// handleResetPassword 重置密码
func (h *Handler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	// 检测重置文件是否存在
	if _, err := os.Stat(resetPasswordFile); os.IsNotExist(err) {
		utils.WriteErrorResponse(w, "ResetFileNotFound",
			"密码重置文件不存在，请先在服务器执行 touch "+resetPasswordFile,
			http.StatusForbidden)
		return
	}

	var req ResetPasswordRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	// 验证新密码
	if req.NewPassword == "" {
		utils.WriteErrorResponse(w, "InvalidParameter", "新密码不能为空", http.StatusBadRequest)
		return
	}
	// 使用统一的密码复杂度验证
	if err := storage.ValidatePassword(req.NewPassword); err != nil {
		utils.WriteErrorResponse(w, "InvalidParameter", err.Error(), http.StatusBadRequest)
		return
	}

	// 更新密码
	if err := h.metadata.SetAdminPassword(req.NewPassword); err != nil {
		utils.Error("重置密码失败", "error", err)
		utils.WriteErrorResponse(w, "InternalError", "重置密码失败", http.StatusInternalServerError)
		return
	}

	// 删除重置文件
	os.Remove(resetPasswordFile)

	// 清除所有会话（强制重新登录）
	sessionStore.mu.Lock()
	sessionStore.sessions = make(map[string]*Session)
	sessionStore.mu.Unlock()

	// 记录密码重置审计日志
	h.Audit(r, storage.AuditActionPasswordReset, "admin", "", true, nil)

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success":    true,
		"message":    "密码重置成功，请使用新密码登录",
		"reset_time": time.Now().UTC().Format(time.RFC3339),
	})
}

// ReloadConfigFromDB 从数据库重新加载配置到全局变量
func ReloadConfigFromDB(metadata *storage.MetadataStore) {
	config.LoadFromDB(metadata)
	// 重新加载 API Key 缓存
	auth.ReloadAPIKeyCache()
}
