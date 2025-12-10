package config

import (
	"strconv"
)

// Config 运行时配置（不再从 YAML 加载，全部从命令行参数和数据库获取）
type Config struct {
	Server   ServerConfig
	Storage  StorageConfig
	Auth     AuthConfig
	Security SecurityConfig
	Log      LogConfig
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	CORSOrigin     string // CORS 允许的来源，默认 "*"
	PresignScheme  string // 预签名URL协议，"http" 或 "https"，默认 "http"
	TrustedProxies string // 信任的代理 IP/CIDR，逗号分隔（如 Cloudflare IP 范围）
}

// ServerConfig 服务器配置（启动时通过命令行参数设置，运行时不可改）
type ServerConfig struct {
	Host   string // 监听地址，命令行参数
	Port   int    // 监听端口，命令行参数
	Region string // S3 区域，可在线修改
}

// StorageConfig 存储配置
type StorageConfig struct {
	DataPath      string // 数据目录，命令行参数（运行时不可改）
	DBPath        string // 数据库路径，命令行参数（运行时不可改）
	MaxObjectSize int64  // 最大对象大小，可在线修改
	MaxUploadSize int64  // 最大上传大小，可在线修改
}

// AuthConfig 认证配置
type AuthConfig struct {
	AdminUsername   string // 管理员用户名
	AccessKeyID     string // 默认 API Key ID
	SecretAccessKey string // 默认 API Key Secret
	PasswordHashed  bool   // 密码是否已哈希（从数据库加载时为 true）
}

// LogConfig 日志配置
type LogConfig struct {
	Level string
}

// Global 全局配置实例
var Global *Config

// NewDefault 创建默认配置
func NewDefault() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Host:   "0.0.0.0",
			Port:   8080,
			Region: "us-east-1",
		},
		Storage: StorageConfig{
			DataPath:      "./data/buckets",
			DBPath:        "./data/metadata.db",
			MaxObjectSize: 5 * 1024 * 1024 * 1024, // 5GB
			MaxUploadSize: 1024 * 1024 * 1024,     // 1GB
		},
		Auth: AuthConfig{
			AdminUsername: "admin",
		},
		Security: SecurityConfig{
			CORSOrigin:     "*",    // 默认允许所有来源
			PresignScheme:  "http", // 默认 HTTP
			TrustedProxies: "",     // 默认不信任任何代理
		},
		Log: LogConfig{
			Level: "info",
		},
	}
	Global = cfg
	return cfg
}

// SettingsLoader 数据库配置加载接口
type SettingsLoader interface {
	GetSetting(key string) (string, error)
	IsInstalled() bool
	GetAdminUsername() string
	VerifyAdminPassword(password string) bool
	GetAuthConfig() (accessKeyID, secretAccessKey string)
	GetStorageConfig() (dataPath string, maxObjectSize, maxUploadSize int64)
}

// LoadFromDB 从数据库加载可修改的配置（Region、存储限制等）
// 注意：Host、Port、DataPath、DBPath 保持命令行参数值，不从数据库覆盖
func LoadFromDB(loader SettingsLoader) {
	if Global == nil {
		Global = NewDefault()
	}

	// 如果系统已安装，从数据库加载配置
	if loader.IsInstalled() {
		// 只加载 Region（Host/Port 由命令行参数决定）
		if region, err := loader.GetSetting("server.region"); err == nil && region != "" {
			Global.Server.Region = region
		}

		// 存储配置（只加载大小限制，DataPath 由命令行参数决定）
		_, maxObjSize, maxUploadSize := loader.GetStorageConfig()
		if maxObjSize > 0 {
			Global.Storage.MaxObjectSize = maxObjSize
		}
		if maxUploadSize > 0 {
			Global.Storage.MaxUploadSize = maxUploadSize
		}

		// 安全配置
		if corsOrigin, err := loader.GetSetting("security.cors_origin"); err == nil && corsOrigin != "" {
			Global.Security.CORSOrigin = corsOrigin
		}
		if presignScheme, err := loader.GetSetting("security.presign_scheme"); err == nil && presignScheme != "" {
			Global.Security.PresignScheme = presignScheme
		}
		if trustedProxies, err := loader.GetSetting("security.trusted_proxies"); err == nil {
			Global.Security.TrustedProxies = trustedProxies
		}

		// 认证配置
		Global.Auth.AdminUsername = loader.GetAdminUsername()
		Global.Auth.PasswordHashed = true

		// API Key
		accessKeyID, secretAccessKey := loader.GetAuthConfig()
		if accessKeyID != "" {
			Global.Auth.AccessKeyID = accessKeyID
			Global.Auth.SecretAccessKey = secretAccessKey
		}
	}
}

// UpdateFromSettings 从数据库设置更新运行时配置
func UpdateFromSettings(settings map[string]string) {
	if Global == nil {
		Global = NewDefault()
	}

	// 只更新运行时可修改的配置
	if v, ok := settings["server.region"]; ok && v != "" {
		Global.Server.Region = v
	}
	if v, ok := settings["storage.max_object_size"]; ok && v != "" {
		if size, err := strconv.ParseInt(v, 10, 64); err == nil && size > 0 {
			Global.Storage.MaxObjectSize = size
		}
	}
	if v, ok := settings["storage.max_upload_size"]; ok && v != "" {
		if size, err := strconv.ParseInt(v, 10, 64); err == nil && size > 0 {
			Global.Storage.MaxUploadSize = size
		}
	}
	if v, ok := settings["auth.admin_username"]; ok && v != "" {
		Global.Auth.AdminUsername = v
	}
	if v, ok := settings["auth.access_key_id"]; ok && v != "" {
		Global.Auth.AccessKeyID = v
	}
	if v, ok := settings["auth.secret_access_key"]; ok && v != "" {
		Global.Auth.SecretAccessKey = v
	}
}
