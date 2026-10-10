package admin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"sss/internal/auth"
	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// 密码重置文件路径
// resetPasswordFile 密码重置标记文件路径（可在测试中修改）
var resetPasswordFile = "./data/.reset_password"

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
	FileExists bool `json:"file_exists"`
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
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	value, err := h.metadata.GetSetting(storage.SettingSystemInstalled)
	if err != nil {
		writeAdminError(w, utils.ErrInternalError, 500, "")
		return
	}
	installed := value == "true"
	var installedAt string
	if installed {
		installedAt, _ = h.metadata.GetSetting(storage.SettingSystemInstalledAt)
	}

	utils.WriteJSONResponse(w, SystemStatusResponse{
		Installed:   installed,
		InstalledAt: installedAt,
		Version:     config.Version, // 直接使用编译时常量
	})
}

// handleInstall 处理安装请求
func (h *Handler) handleInstall(w http.ResponseWriter, r *http.Request) {
	setupMu.Lock()
	defer setupMu.Unlock()
	if r.Method != http.MethodPost {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	// 检查是否已安装
	if h.metadata.IsInstalled() {
		utils.WriteErrorResponse(w, "AlreadyInstalled", "系统已安装，无法重复安装", http.StatusBadRequest)
		return
	}

	var req SetupRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		writeAdminError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
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

	// 初始化配置（返回生成的 API Key）
	result, err := h.metadata.InstallSystem(req.AdminUsername, req.AdminPassword, req.ServerRegion)
	if err != nil {
		utils.Error("初始化配置失败", "error", err)
		utils.WriteErrorResponse(w, "InternalError", "初始化配置失败", http.StatusInternalServerError)
		return
	}

	// 重新加载配置到全局
	h.filestore.MutationMu.Lock()
	ReloadConfigFromDB(h.metadata)
	h.filestore.MutationMu.Unlock()

	// 记录安装审计日志
	h.Audit(r, storage.AuditActionSystemInstall, req.AdminUsername, "", true, map[string]string{
		"server_host": req.ServerHost,
		"server_port": req.ServerPort,
	})

	utils.WriteJSONResponse(w, SetupResponse{
		Success:         true,
		Message:         "安装成功",
		AccessKeyID:     result.AccessKeyID,
		SecretAccessKey: result.SecretAccessKey,
	})
}

// handleResetPasswordCheck 检测密码重置文件是否存在
func (h *Handler) handleResetPasswordCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	setupMu.Lock()
	defer setupMu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	token, _, err := readRecoveryMarker(false)
	available := err == nil
	if available && token != "" {
		used, lookupErr := h.metadata.RecoveryConsumed(token)
		if lookupErr != nil {
			writeAdminError(w, utils.ErrInternalError, 500, "")
			return
		}
		available = !used
	}
	utils.WriteJSONResponse(w, ResetPasswordCheckResponse{FileExists: available})
}

// handleResetPassword 重置密码
func (h *Handler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	// Do not hold the recovery mutex while an untrusted client sends JSON.

	var req ResetPasswordRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		writeAdminError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
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

	setupMu.Lock()
	defer setupMu.Unlock()
	token, markerInfo, err := readRecoveryMarker(true)
	if err != nil {
		utils.WriteErrorResponse(w, "ResetFileNotFound", "Local recovery authorization is unavailable; contact the server operator", http.StatusForbidden)
		return
	}
	if err := h.metadata.ResetAdminPassword(token, req.NewPassword); err != nil {
		if errors.Is(err, storage.ErrRecoveryConsumed) {
			utils.WriteErrorResponse(w, "ResetFileNotFound", "Local recovery authorization has already been used", http.StatusForbidden)
		} else {
			utils.Error("重置密码失败", "error", err)
			utils.WriteErrorResponse(w, "InternalError", "Password reset failed; the local authorization remains available for retry", http.StatusInternalServerError)
		}
		return
	}
	// Never remove a newly replaced operator marker. A leftover consumed marker
	// is harmless, including when cleanup fails or the process dies here.
	if current, err := os.Lstat(resetPasswordFile); err == nil && os.SameFile(markerInfo, current) {
		if err := os.Remove(resetPasswordFile); err != nil {
			utils.Warn("Recovery marker cleanup failed", "error", err)
		}
	}

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

var setupMu sync.Mutex

// Empty markers created by existing `touch data/.reset_password` instructions
// are upgraded to a durable random nonce before any credential change. Never
// follow symlinks or silently replace malformed/nonempty operator files.
func readRecoveryMarker(initialize bool) (string, os.FileInfo, error) {
	info, err := os.Lstat(resetPasswordFile)
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64 {
		return "", nil, errors.New("invalid recovery marker")
	}
	flags := os.O_RDONLY
	if initialize {
		flags = os.O_RDWR
	}
	file, err := os.OpenFile(resetPasswordFile, flags, 0)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil || !os.SameFile(info, actual) {
		return "", nil, errors.New("recovery marker changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, 65))
	if err != nil {
		return "", nil, err
	}
	if len(data) == 0 && initialize {
		nonce := make([]byte, 32)
		if _, err := rand.Read(nonce); err != nil {
			return "", nil, err
		}
		data = []byte(hex.EncodeToString(nonce))
		if _, err := file.WriteAt(data, 0); err != nil {
			return "", nil, err
		}
	}
	if len(data) != 0 {
		if len(data) != 64 {
			return "", nil, errors.New("invalid recovery marker")
		}
		if _, err := hex.DecodeString(string(data)); err != nil {
			return "", nil, err
		}
	}
	if initialize {
		// Also sync on retries: a previous request may have written the nonce but
		// failed to flush it. Never commit its consumption until it is durable.
		if err := file.Sync(); err != nil {
			return "", nil, err
		}
	}
	return string(data), info, nil
}
