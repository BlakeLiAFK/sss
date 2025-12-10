package storage

import (
	"database/sql"
	"errors"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// 密码验证错误
var (
	ErrPasswordTooShort = errors.New("密码长度至少为 8 个字符")
	ErrPasswordNoUpper  = errors.New("密码必须包含至少一个大写字母")
	ErrPasswordNoLower  = errors.New("密码必须包含至少一个小写字母")
	ErrPasswordNoDigit  = errors.New("密码必须包含至少一个数字")
)

// SystemSetting 系统配置项
type SystemSetting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 配置键常量
const (
	// 系统状态
	SettingSystemInstalled   = "system.installed"
	SettingSystemInstalledAt = "system.installed_at"
	SettingSystemVersion     = "system.version"

	// 服务器配置
	SettingServerHost   = "server.host"
	SettingServerPort   = "server.port"
	SettingServerRegion = "server.region"

	// 存储配置
	SettingStorageDataPath      = "storage.data_path"
	SettingStorageMaxObjectSize = "storage.max_object_size"
	SettingStorageMaxUploadSize = "storage.max_upload_size"

	// 安全配置
	SettingSecurityCORSOrigin     = "security.cors_origin"      // CORS 允许的来源，默认 "*"
	SettingSecurityPresignScheme  = "security.presign_scheme"   // 预签名URL协议，"http" 或 "https"
	SettingSecurityTrustedProxies = "security.trusted_proxies"  // 信任的代理 IP/CIDR，逗号分隔

	// 认证配置
	SettingAuthAdminUsername     = "auth.admin_username"
	SettingAuthAdminPasswordHash = "auth.admin_password_hash"

	// 旧版兼容配置（API Key）
	SettingAuthAccessKeyID     = "auth.access_key_id"
	SettingAuthSecretAccessKey = "auth.secret_access_key"

	// GeoStats 配置
	SettingGeoStatsEnabled       = "geo_stats.enabled"        // 是否启用，"true" 或 "false"
	SettingGeoStatsMode          = "geo_stats.mode"           // 写入模式，"realtime" 或 "batch"
	SettingGeoStatsBatchSize     = "geo_stats.batch_size"     // 批量模式缓存大小
	SettingGeoStatsFlushInterval = "geo_stats.flush_interval" // 批量模式刷新间隔（秒）
	SettingGeoStatsRetentionDays = "geo_stats.retention_days" // 数据保留天数
)

// initSettingsTable 初始化系统配置表
func (m *MetadataStore) initSettingsTable() error {
	schema := `CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	)`
	_, err := m.db.Exec(schema)
	return err
}

// GetSetting 获取配置项
func (m *MetadataStore) GetSetting(key string) (string, error) {
	var value string
	err := m.db.QueryRow("SELECT value FROM system_settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting 设置配置项
func (m *MetadataStore) SetSetting(key, value string) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec(`
			INSERT OR REPLACE INTO system_settings (key, value, updated_at)
			VALUES (?, ?, ?)`,
			key, value, time.Now().UTC(),
		)
		return err
	})
}

// GetSettings 批量获取配置项
func (m *MetadataStore) GetSettings(keys []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, key := range keys {
		value, err := m.GetSetting(key)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

// GetAllSettings 获取所有配置项
func (m *MetadataStore) GetAllSettings() ([]SystemSetting, error) {
	rows, err := m.db.Query("SELECT key, value, updated_at FROM system_settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []SystemSetting
	for rows.Next() {
		var s SystemSetting
		if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, nil
}

// IsInstalled 检查系统是否已安装
func (m *MetadataStore) IsInstalled() bool {
	value, err := m.GetSetting(SettingSystemInstalled)
	if err != nil {
		return false
	}
	return value == "true"
}

// SetInstalled 设置系统为已安装状态
func (m *MetadataStore) SetInstalled() error {
	if err := m.SetSetting(SettingSystemInstalled, "true"); err != nil {
		return err
	}
	return m.SetSetting(SettingSystemInstalledAt, time.Now().UTC().Format(time.RFC3339))
}

// ValidatePassword 验证密码复杂度
// 要求：至少 8 个字符，包含大写字母、小写字母和数字
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}

	if !hasUpper {
		return ErrPasswordNoUpper
	}
	if !hasLower {
		return ErrPasswordNoLower
	}
	if !hasDigit {
		return ErrPasswordNoDigit
	}

	return nil
}

// SetAdminPassword 设置管理员密码（验证复杂度后 bcrypt 哈希）
func (m *MetadataStore) SetAdminPassword(password string) error {
	// 验证密码复杂度
	if err := ValidatePassword(password); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return m.SetSetting(SettingAuthAdminPasswordHash, string(hash))
}

// VerifyAdminPassword 验证管理员密码
func (m *MetadataStore) VerifyAdminPassword(password string) bool {
	hash, err := m.GetSetting(SettingAuthAdminPasswordHash)
	if err != nil || hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GetAdminUsername 获取管理员用户名
func (m *MetadataStore) GetAdminUsername() string {
	username, err := m.GetSetting(SettingAuthAdminUsername)
	if err != nil || username == "" {
		return "admin" // 默认用户名
	}
	return username
}

// InitDefaultSettings 初始化默认配置（安装时调用）
func (m *MetadataStore) InitDefaultSettings(adminUsername, adminPassword string) error {
	// 服务器配置
	defaults := map[string]string{
		SettingServerHost:           "0.0.0.0",
		SettingServerPort:           "8080",
		SettingServerRegion:         "us-east-1",
		SettingStorageDataPath:      "./data/buckets",
		SettingStorageMaxObjectSize: "5368709120", // 5GB
		SettingStorageMaxUploadSize: "1073741824", // 1GB
		SettingAuthAdminUsername:    adminUsername,
		SettingSystemVersion:        "1.1.0",
	}

	for key, value := range defaults {
		if err := m.SetSetting(key, value); err != nil {
			return err
		}
	}

	// 设置密码（bcrypt 哈希）
	if err := m.SetAdminPassword(adminPassword); err != nil {
		return err
	}

	return nil
}

// GetServerConfig 获取服务器配置
func (m *MetadataStore) GetServerConfig() (host string, port int, region string) {
	host, _ = m.GetSetting(SettingServerHost)
	if host == "" {
		host = "0.0.0.0"
	}

	portStr, _ := m.GetSetting(SettingServerPort)
	port = 8080
	if portStr != "" {
		var p int
		if _, err := parseIntSafe(portStr, &p); err == nil && p > 0 {
			port = p
		}
	}

	region, _ = m.GetSetting(SettingServerRegion)
	if region == "" {
		region = "us-east-1"
	}
	return
}

// GetStorageConfig 获取存储配置
func (m *MetadataStore) GetStorageConfig() (dataPath string, maxObjectSize, maxUploadSize int64) {
	dataPath, _ = m.GetSetting(SettingStorageDataPath)
	if dataPath == "" {
		dataPath = "./data/buckets"
	}

	maxObjectSizeStr, _ := m.GetSetting(SettingStorageMaxObjectSize)
	maxObjectSize = 5 * 1024 * 1024 * 1024 // 5GB 默认
	if maxObjectSizeStr != "" {
		var size int64
		if _, err := parseInt64Safe(maxObjectSizeStr, &size); err == nil && size > 0 {
			maxObjectSize = size
		}
	}

	maxUploadSizeStr, _ := m.GetSetting(SettingStorageMaxUploadSize)
	maxUploadSize = 1024 * 1024 * 1024 // 1GB 默认
	if maxUploadSizeStr != "" {
		var size int64
		if _, err := parseInt64Safe(maxUploadSizeStr, &size); err == nil && size > 0 {
			maxUploadSize = size
		}
	}
	return
}

// GetAuthConfig 获取认证配置
func (m *MetadataStore) GetAuthConfig() (accessKeyID, secretAccessKey string) {
	accessKeyID, _ = m.GetSetting(SettingAuthAccessKeyID)
	secretAccessKey, _ = m.GetSetting(SettingAuthSecretAccessKey)
	return
}

// 辅助函数：安全解析整数
func parseIntSafe(s string, result *int) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	*result = n
	return n, nil
}

func parseInt64Safe(s string, result *int64) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int64(c-'0')
	}
	*result = n
	return n, nil
}
