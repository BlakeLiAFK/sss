package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// SettingsResponse 系统设置响应
type SettingsResponse struct {
	Runtime  RuntimeSettings  `json:"runtime"`  // 运行时参数（只读）
	Storage  StorageSettings  `json:"storage"`  // 存储设置（可修改）
	Security SecuritySettings `json:"security"` // 安全设置（可修改）
	System   SystemInfo       `json:"system"`   // 系统信息（只读）
}

// SecuritySettings 安全设置（可在线修改）
type SecuritySettings struct {
	CORSOrigin     string `json:"cors_origin"`     // CORS 允许的来源，默认 "*"
	PresignScheme  string `json:"presign_scheme"`  // 预签名URL协议，"http" 或 "https"
	TrustedProxies string `json:"trusted_proxies"` // 信任的代理 IP/CIDR，逗号分隔
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
	GitCommit   string `json:"git_commit,omitempty"`
	BuildTime   string `json:"build_time,omitempty"`
}

// handleSettings 处理系统设置 API
func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSettings(w, r)
	case http.MethodPut:
		h.updateSettings(w, r)
	default:
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
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

	// 安全设置（可在线修改）
	security := SecuritySettings{
		CORSOrigin:     config.Global.Security.CORSOrigin,
		PresignScheme:  config.Global.Security.PresignScheme,
		TrustedProxies: config.Global.Security.TrustedProxies,
	}
	// 确保有默认值
	if security.CORSOrigin == "" {
		security.CORSOrigin = "*"
	}
	if security.PresignScheme == "" {
		security.PresignScheme = "http"
	}

	// 系统信息
	installedAt, _ := h.metadata.GetSetting(storage.SettingSystemInstalledAt)

	resp := SettingsResponse{
		Runtime:  runtime,
		Storage:  storage_,
		Security: security,
		System: SystemInfo{
			Installed:   h.metadata.IsInstalled(),
			InstalledAt: installedAt,
			Version:     config.Version,
			GitCommit:   config.GitCommit,
			BuildTime:   config.BuildTime,
		},
	}

	utils.WriteJSONResponse(w, resp)
}

// UpdateSettingsRequest 更新设置请求（只包含可修改的字段）
type UpdateSettingsRequest struct {
	Region         *string `json:"region,omitempty"`
	MaxObjectSize  *int64  `json:"max_object_size,omitempty"`
	MaxUploadSize  *int64  `json:"max_upload_size,omitempty"`
	CORSOrigin     *string `json:"cors_origin,omitempty"`
	PresignScheme  *string `json:"presign_scheme,omitempty"`
	TrustedProxies *string `json:"trusted_proxies,omitempty"`
}

// updateSettings 更新系统设置
func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", "Invalid JSON", http.StatusBadRequest)
		return
	}

	values := map[string]string{}
	if req.Region != nil {
		if strings.TrimSpace(*req.Region) == "" {
			writeAdminError(w, utils.ErrInvalidArgument, 400, "")
			return
		}
		values[storage.SettingServerRegion] = *req.Region
	}
	for key, n := range map[string]*int64{storage.SettingStorageMaxObjectSize: req.MaxObjectSize, storage.SettingStorageMaxUploadSize: req.MaxUploadSize} {
		if n != nil {
			if *n <= 0 || *n > 5<<30 {
				writeAdminError(w, utils.ErrInvalidArgument, 400, "")
				return
			}
			values[key] = strconv.FormatInt(*n, 10)
		}
	}
	if req.PresignScheme != nil {
		if *req.PresignScheme != "http" && *req.PresignScheme != "https" {
			writeAdminError(w, utils.ErrInvalidArgument, 400, "")
			return
		}
		values[storage.SettingSecurityPresignScheme] = *req.PresignScheme
	}
	if req.CORSOrigin != nil {
		if strings.ContainsAny(*req.CORSOrigin, "\r\n") {
			writeAdminError(w, utils.ErrInvalidArgument, 400, "")
			return
		}
		values[storage.SettingSecurityCORSOrigin] = *req.CORSOrigin
	}
	if req.TrustedProxies != nil {
		for _, cidr := range strings.Split(*req.TrustedProxies, ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr != "" {
				if _, err := netip.ParsePrefix(cidr); err != nil {
					if _, err := netip.ParseAddr(cidr); err != nil {
						writeAdminError(w, utils.ErrInvalidArgument, 400, "")
						return
					}
				}
			}
		}
		values[storage.SettingSecurityTrustedProxies] = *req.TrustedProxies
	}
	if err := h.metadata.SetSettings(values); err != nil {
		writeAdminError(w, utils.ErrInternalError, 500, "")
		return
	}
	if req.Region != nil {
		config.Global.Server.Region = *req.Region
	}
	if req.MaxObjectSize != nil {
		config.Global.Storage.MaxObjectSize = *req.MaxObjectSize
	}
	if req.MaxUploadSize != nil {
		config.Global.Storage.MaxUploadSize = *req.MaxUploadSize
	}
	if req.PresignScheme != nil {
		config.Global.Security.PresignScheme = *req.PresignScheme
	}
	if req.CORSOrigin != nil {
		config.Global.Security.CORSOrigin = *req.CORSOrigin
	}
	if req.TrustedProxies != nil {
		config.Global.Security.TrustedProxies = *req.TrustedProxies
		utils.ReloadTrustedProxies(*req.TrustedProxies)
	}
	// 记录审计日志
	h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "system", true, "更新系统设置")

	// 返回更新后的设置
	h.getSettings(w, r)
}

// handleChangePassword 修改管理员密码
func (h *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
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

	if req.OldPassword == "" || req.NewPassword == "" {
		utils.WriteErrorResponse(w, "InvalidRequest", "密码不能为空", http.StatusBadRequest)
		return
	}

	// 使用统一的密码复杂度验证
	if err := storage.ValidatePassword(req.NewPassword); err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", err.Error(), http.StatusBadRequest)
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

	sessionStore.mu.Lock()
	sessionStore.sessions = make(map[string]*Session)
	sessionStore.mu.Unlock()
	// 记录审计日志
	h.Audit(r, storage.AuditActionPasswordChange, "admin", "system", true, "管理员密码已更改")

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "密码修改成功",
	})
}

// handleGeoIP 处理 GeoIP 数据库管理
func (h *Handler) handleGeoIP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getGeoIPStatus(w, r)
	case http.MethodPost:
		h.uploadGeoIP(w, r)
	case http.MethodDelete:
		h.deleteGeoIP(w, r)
	default:
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// GeoIPStatusResponse GeoIP 状态响应
type GeoIPStatusResponse struct {
	Enabled bool   `json:"enabled"` // 是否启用
	Path    string `json:"path"`    // 数据库路径
}

// getGeoIPStatus 获取 GeoIP 状态
func (h *Handler) getGeoIPStatus(w http.ResponseWriter, r *http.Request) {
	geoIP := utils.GetGeoIPService()
	resp := GeoIPStatusResponse{
		Enabled: geoIP.IsEnabled(),
		Path:    utils.GetDefaultGeoIPPath(config.Global.Storage.DBPath),
	}
	utils.WriteJSONResponse(w, resp)
}

// uploadGeoIP 上传 GeoIP 数据库
func (h *Handler) uploadGeoIP(w http.ResponseWriter, r *http.Request) {
	// 限制上传大小 (100MB)
	r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024)

	// 解析 multipart form
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", "文件过大或格式错误", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", "未找到上传文件", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 验证文件扩展名
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".mmdb") {
		utils.WriteErrorResponse(w, "InvalidRequest", "仅支持 .mmdb 格式的 GeoIP 数据库", http.StatusBadRequest)
		return
	}

	// 确保目录存在
	geoIPPath := utils.GetDefaultGeoIPPath(config.Global.Storage.DBPath)
	geoIPDir := filepath.Dir(geoIPPath)
	if err := os.MkdirAll(geoIPDir, 0755); err != nil {
		utils.WriteErrorResponse(w, "InternalError", "创建目录失败", http.StatusInternalServerError)
		return
	}

	// 先保存到临时文件
	outFile, err := os.CreateTemp(geoIPDir, ".geoip-*")
	if err != nil {
		utils.WriteErrorResponse(w, "InternalError", "创建文件失败", http.StatusInternalServerError)
		return
	}

	tempPath := outFile.Name()
	defer os.Remove(tempPath)
	_, err = io.Copy(outFile, file)
	outFile.Close()
	if err != nil {
		os.Remove(tempPath)
		utils.WriteErrorResponse(w, "InternalError", "保存文件失败", http.StatusInternalServerError)
		return
	}

	if err := utils.GetGeoIPService().Replace(tempPath, geoIPPath); err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", "Invalid database or replacement failed", 400)
		return
	}
	// 记录审计日志
	h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "geoip", true, "上传 GeoIP 数据库")

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "GeoIP 数据库已上传并启用",
	})
}

// deleteGeoIP 删除 GeoIP 数据库
func (h *Handler) deleteGeoIP(w http.ResponseWriter, r *http.Request) {
	geoIPPath := utils.GetDefaultGeoIPPath(config.Global.Storage.DBPath)

	// 关闭并禁用 GeoIP 服务
	geoIP := utils.GetGeoIPService()

	// 删除文件
	if err := os.Remove(geoIPPath); err != nil && !os.IsNotExist(err) {
		utils.WriteErrorResponse(w, "InternalError", "删除文件失败", http.StatusInternalServerError)
		return
	}

	geoIP.Close()
	// 记录审计日志
	h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "geoip", true, "删除 GeoIP 数据库")

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "GeoIP 数据库已删除",
	})
}

// handleCheckUpdate 检查版本更新
func (h *Handler) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	result, err := utils.CheckForUpdate()
	if err != nil {
		utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteJSONResponse(w, result)
}
