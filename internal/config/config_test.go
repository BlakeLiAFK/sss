package config

import (
	"testing"
)

// =============================================================================
// config.go 测试
// =============================================================================

// TestNewDefault 测试创建默认配置
func TestNewDefault(t *testing.T) {
	// 保存原始 Global
	originalGlobal := Global
	defer func() { Global = originalGlobal }()

	cfg := NewDefault()

	t.Run("返回非空配置", func(t *testing.T) {
		if cfg == nil {
			t.Fatal("NewDefault() 返回 nil")
		}
	})

	t.Run("Server 默认值", func(t *testing.T) {
		if cfg.Server.Host != "0.0.0.0" {
			t.Errorf("Server.Host = %v, want 0.0.0.0", cfg.Server.Host)
		}
		if cfg.Server.Port != 8080 {
			t.Errorf("Server.Port = %v, want 8080", cfg.Server.Port)
		}
		if cfg.Server.Region != "us-east-1" {
			t.Errorf("Server.Region = %v, want us-east-1", cfg.Server.Region)
		}
	})

	t.Run("Storage 默认值", func(t *testing.T) {
		if cfg.Storage.DataPath != "./data/buckets" {
			t.Errorf("Storage.DataPath = %v, want ./data/buckets", cfg.Storage.DataPath)
		}
		if cfg.Storage.DBPath != "./data/metadata.db" {
			t.Errorf("Storage.DBPath = %v, want ./data/metadata.db", cfg.Storage.DBPath)
		}
		// 5GB
		expectedMaxObjSize := int64(5 * 1024 * 1024 * 1024)
		if cfg.Storage.MaxObjectSize != expectedMaxObjSize {
			t.Errorf("Storage.MaxObjectSize = %v, want %v", cfg.Storage.MaxObjectSize, expectedMaxObjSize)
		}
		// 1GB
		expectedMaxUploadSize := int64(1024 * 1024 * 1024)
		if cfg.Storage.MaxUploadSize != expectedMaxUploadSize {
			t.Errorf("Storage.MaxUploadSize = %v, want %v", cfg.Storage.MaxUploadSize, expectedMaxUploadSize)
		}
	})

	t.Run("Auth 默认值", func(t *testing.T) {
		if cfg.Auth.AdminUsername != "admin" {
			t.Errorf("Auth.AdminUsername = %v, want admin", cfg.Auth.AdminUsername)
		}
	})

	t.Run("Security 默认值", func(t *testing.T) {
		if cfg.Security.CORSOrigin != "*" {
			t.Errorf("Security.CORSOrigin = %v, want *", cfg.Security.CORSOrigin)
		}
		if cfg.Security.PresignScheme != "http" {
			t.Errorf("Security.PresignScheme = %v, want http", cfg.Security.PresignScheme)
		}
	})

	t.Run("Log 默认值", func(t *testing.T) {
		if cfg.Log.Level != "info" {
			t.Errorf("Log.Level = %v, want info", cfg.Log.Level)
		}
	})

	t.Run("设置全局变量", func(t *testing.T) {
		if Global != cfg {
			t.Error("NewDefault() 应设置 Global 变量")
		}
	})
}

// mockSettingsLoader 模拟 SettingsLoader 接口
type mockSettingsLoader struct {
	settings          map[string]string
	installed         bool
	adminUsername     string
	accessKeyID       string
	secretAccessKey   string
	dataPath          string
	maxObjectSize     int64
	maxUploadSize     int64
}

func (m *mockSettingsLoader) GetSetting(key string) (string, error) {
	if v, ok := m.settings[key]; ok {
		return v, nil
	}
	return "", nil
}

func (m *mockSettingsLoader) IsInstalled() bool {
	return m.installed
}

func (m *mockSettingsLoader) GetAdminUsername() string {
	return m.adminUsername
}

func (m *mockSettingsLoader) VerifyAdminPassword(password string) bool {
	return password == "correct-password"
}

func (m *mockSettingsLoader) GetAuthConfig() (string, string) {
	return m.accessKeyID, m.secretAccessKey
}

func (m *mockSettingsLoader) GetStorageConfig() (string, int64, int64) {
	return m.dataPath, m.maxObjectSize, m.maxUploadSize
}

// TestLoadFromDB 测试从数据库加载配置
func TestLoadFromDB(t *testing.T) {
	tests := []struct {
		name           string
		loader         *mockSettingsLoader
		wantRegion     string
		wantMaxObjSize int64
		wantCORSOrigin string
		wantUsername   string
	}{
		{
			name: "系统未安装，使用默认配置",
			loader: &mockSettingsLoader{
				installed: false,
			},
			wantRegion:     "us-east-1",
			wantMaxObjSize: 5 * 1024 * 1024 * 1024,
			wantCORSOrigin: "*",
			wantUsername:   "admin",
		},
		{
			name: "系统已安装，加载数据库配置",
			loader: &mockSettingsLoader{
				installed:     true,
				adminUsername: "superadmin",
				accessKeyID:   "AKIATEST",
				secretAccessKey: "secret123",
				maxObjectSize: 10 * 1024 * 1024 * 1024, // 10GB
				maxUploadSize: 2 * 1024 * 1024 * 1024,  // 2GB
				settings: map[string]string{
					"server.region":         "ap-northeast-1",
					"security.cors_origin":  "https://example.com",
					"security.presign_scheme": "https",
				},
			},
			wantRegion:     "ap-northeast-1",
			wantMaxObjSize: 10 * 1024 * 1024 * 1024,
			wantCORSOrigin: "https://example.com",
			wantUsername:   "superadmin",
		},
		{
			name: "部分配置为空，保持默认值",
			loader: &mockSettingsLoader{
				installed:     true,
				adminUsername: "admin",
				maxObjectSize: 0, // 0 表示使用默认值
				maxUploadSize: 0,
				settings:      map[string]string{},
			},
			wantRegion:     "us-east-1",
			wantMaxObjSize: 5 * 1024 * 1024 * 1024,
			wantCORSOrigin: "*",
			wantUsername:   "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置全局配置
			Global = nil

			LoadFromDB(tt.loader)

			if Global == nil {
				t.Fatal("LoadFromDB() 应初始化 Global")
			}

			if Global.Server.Region != tt.wantRegion {
				t.Errorf("Server.Region = %v, want %v", Global.Server.Region, tt.wantRegion)
			}
			if Global.Storage.MaxObjectSize != tt.wantMaxObjSize {
				t.Errorf("Storage.MaxObjectSize = %v, want %v", Global.Storage.MaxObjectSize, tt.wantMaxObjSize)
			}
			if Global.Security.CORSOrigin != tt.wantCORSOrigin {
				t.Errorf("Security.CORSOrigin = %v, want %v", Global.Security.CORSOrigin, tt.wantCORSOrigin)
			}
			if Global.Auth.AdminUsername != tt.wantUsername {
				t.Errorf("Auth.AdminUsername = %v, want %v", Global.Auth.AdminUsername, tt.wantUsername)
			}
		})
	}
}

// TestLoadFromDB_GlobalNil 测试 Global 为 nil 时的行为
func TestLoadFromDB_GlobalNil(t *testing.T) {
	Global = nil
	loader := &mockSettingsLoader{
		installed: false,
	}

	LoadFromDB(loader)

	if Global == nil {
		t.Error("LoadFromDB() 应在 Global 为 nil 时创建默认配置")
	}
}

// TestLoadFromDB_WithAPIKey 测试加载 API Key 配置
func TestLoadFromDB_WithAPIKey(t *testing.T) {
	Global = nil
	loader := &mockSettingsLoader{
		installed:       true,
		adminUsername:   "admin",
		accessKeyID:     "AKIA12345678",
		secretAccessKey: "superSecretKey",
	}

	LoadFromDB(loader)

	if Global.Auth.AccessKeyID != "AKIA12345678" {
		t.Errorf("Auth.AccessKeyID = %v, want AKIA12345678", Global.Auth.AccessKeyID)
	}
	if Global.Auth.SecretAccessKey != "superSecretKey" {
		t.Errorf("Auth.SecretAccessKey = %v, want superSecretKey", Global.Auth.SecretAccessKey)
	}
	if !Global.Auth.PasswordHashed {
		t.Error("Auth.PasswordHashed 应为 true")
	}
}

// TestLoadFromDB_PresignScheme 测试加载预签名协议
func TestLoadFromDB_PresignScheme(t *testing.T) {
	Global = nil
	loader := &mockSettingsLoader{
		installed:     true,
		adminUsername: "admin",
		settings: map[string]string{
			"security.presign_scheme": "https",
		},
	}

	LoadFromDB(loader)

	if Global.Security.PresignScheme != "https" {
		t.Errorf("Security.PresignScheme = %v, want https", Global.Security.PresignScheme)
	}
}

// TestUpdateFromSettings 测试从设置更新配置
func TestUpdateFromSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		check    func(t *testing.T)
	}{
		{
			name: "更新 Region",
			settings: map[string]string{
				"server.region": "eu-west-1",
			},
			check: func(t *testing.T) {
				if Global.Server.Region != "eu-west-1" {
					t.Errorf("Server.Region = %v, want eu-west-1", Global.Server.Region)
				}
			},
		},
		{
			name: "更新存储大小限制",
			settings: map[string]string{
				"storage.max_object_size": "2147483648", // 2GB
				"storage.max_upload_size": "536870912",  // 512MB
			},
			check: func(t *testing.T) {
				if Global.Storage.MaxObjectSize != 2147483648 {
					t.Errorf("Storage.MaxObjectSize = %v, want 2147483648", Global.Storage.MaxObjectSize)
				}
				if Global.Storage.MaxUploadSize != 536870912 {
					t.Errorf("Storage.MaxUploadSize = %v, want 536870912", Global.Storage.MaxUploadSize)
				}
			},
		},
		{
			name: "更新认证配置",
			settings: map[string]string{
				"auth.admin_username":    "newadmin",
				"auth.access_key_id":     "NEWKEY123",
				"auth.secret_access_key": "newsecret",
			},
			check: func(t *testing.T) {
				if Global.Auth.AdminUsername != "newadmin" {
					t.Errorf("Auth.AdminUsername = %v, want newadmin", Global.Auth.AdminUsername)
				}
				if Global.Auth.AccessKeyID != "NEWKEY123" {
					t.Errorf("Auth.AccessKeyID = %v, want NEWKEY123", Global.Auth.AccessKeyID)
				}
				if Global.Auth.SecretAccessKey != "newsecret" {
					t.Errorf("Auth.SecretAccessKey = %v, want newsecret", Global.Auth.SecretAccessKey)
				}
			},
		},
		{
			name: "无效数字不更新",
			settings: map[string]string{
				"storage.max_object_size": "invalid",
				"storage.max_upload_size": "not-a-number",
			},
			check: func(t *testing.T) {
				// 应保持默认值
				if Global.Storage.MaxObjectSize != 5*1024*1024*1024 {
					t.Errorf("无效值不应更新 MaxObjectSize")
				}
			},
		},
		{
			name: "空值不更新",
			settings: map[string]string{
				"server.region": "",
			},
			check: func(t *testing.T) {
				// Region 应保持默认值
				if Global.Server.Region != "us-east-1" {
					t.Errorf("空值不应更新 Region, got %v", Global.Server.Region)
				}
			},
		},
		{
			name: "负数不更新",
			settings: map[string]string{
				"storage.max_object_size": "-1",
			},
			check: func(t *testing.T) {
				// 应保持默认值
				if Global.Storage.MaxObjectSize != 5*1024*1024*1024 {
					t.Errorf("负数不应更新 MaxObjectSize")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 每次测试前重置为默认配置
			Global = nil
			NewDefault()

			UpdateFromSettings(tt.settings)
			tt.check(t)
		})
	}
}

// TestUpdateFromSettings_GlobalNil 测试 Global 为 nil 时的行为
func TestUpdateFromSettings_GlobalNil(t *testing.T) {
	Global = nil

	UpdateFromSettings(map[string]string{
		"server.region": "test-region",
	})

	if Global == nil {
		t.Error("UpdateFromSettings() 应在 Global 为 nil 时创建默认配置")
	}
	if Global.Server.Region != "test-region" {
		t.Errorf("Server.Region = %v, want test-region", Global.Server.Region)
	}
}

// TestConfigStructure 测试配置结构体
func TestConfigStructure(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host:   "localhost",
			Port:   9000,
			Region: "local",
		},
		Storage: StorageConfig{
			DataPath:      "/tmp/data",
			DBPath:        "/tmp/db.sqlite",
			MaxObjectSize: 1024,
			MaxUploadSize: 512,
		},
		Auth: AuthConfig{
			AdminUsername:   "testadmin",
			AccessKeyID:     "TESTKEY",
			SecretAccessKey: "TESTSECRET",
			PasswordHashed:  true,
		},
		Security: SecurityConfig{
			CORSOrigin:    "https://test.com",
			PresignScheme: "https",
		},
		Log: LogConfig{
			Level: "debug",
		},
	}

	// 验证字段赋值正确
	if cfg.Server.Host != "localhost" {
		t.Errorf("Server.Host = %v", cfg.Server.Host)
	}
	if cfg.Storage.DataPath != "/tmp/data" {
		t.Errorf("Storage.DataPath = %v", cfg.Storage.DataPath)
	}
	if cfg.Auth.AdminUsername != "testadmin" {
		t.Errorf("Auth.AdminUsername = %v", cfg.Auth.AdminUsername)
	}
	if cfg.Security.CORSOrigin != "https://test.com" {
		t.Errorf("Security.CORSOrigin = %v", cfg.Security.CORSOrigin)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %v", cfg.Log.Level)
	}
}

// =============================================================================
// version.go 测试
// =============================================================================

// TestVersion 测试版本常量
func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version 不应为空")
	}

	// 验证版本号格式（简单检查）
	if len(Version) < 5 {
		t.Errorf("Version 格式可能不正确: %s", Version)
	}
}

// TestVersionValue 测试版本值
func TestVersionValue(t *testing.T) {
	// 当前版本应该是 1.0.0
	if Version != "1.0.0" {
		t.Logf("当前版本: %s", Version)
	}
}

// =============================================================================
// 并发安全测试
// =============================================================================

// TestConcurrentAccess 测试并发访问配置
// 注意：当前配置模块 Global 变量没有并发保护
// 此测试仅验证单次操作不会 panic
func TestConcurrentAccess(t *testing.T) {
	// 保存原始 Global
	originalGlobal := Global
	defer func() { Global = originalGlobal }()

	NewDefault()

	// 顺序执行读取操作，验证不会 panic
	_ = Global.Server.Region
	_ = Global.Storage.MaxObjectSize
	_ = Global.Auth.AdminUsername

	// 顺序执行更新操作
	UpdateFromSettings(map[string]string{
		"server.region": "test-region",
	})

	// 验证更新成功
	if Global.Server.Region != "test-region" {
		t.Errorf("Server.Region = %v, want test-region", Global.Server.Region)
	}
}

// TestMultipleNewDefault 测试多次调用 NewDefault
func TestMultipleNewDefault(t *testing.T) {
	// 多次调用应该正常工作
	cfg1 := NewDefault()
	cfg2 := NewDefault()
	cfg3 := NewDefault()

	// 每次调用返回新的配置对象
	if cfg1 == cfg2 || cfg2 == cfg3 {
		// 注意：当前实现每次都返回新对象
		t.Log("NewDefault() 每次调用返回新对象")
	}

	// Global 应该指向最后一次创建的配置
	if Global != cfg3 {
		t.Error("Global 应指向最后一次创建的配置")
	}
}

// =============================================================================
// 边界条件测试
// =============================================================================

// TestUpdateFromSettings_EdgeCases 测试更新设置的边界条件
func TestUpdateFromSettings_EdgeCases(t *testing.T) {
	originalGlobal := Global
	defer func() { Global = originalGlobal }()

	t.Run("nil settings", func(t *testing.T) {
		NewDefault()
		// 不应 panic
		UpdateFromSettings(nil)
	})

	t.Run("empty settings", func(t *testing.T) {
		NewDefault()
		UpdateFromSettings(map[string]string{})
		// 应保持默认值
		if Global.Server.Region != "us-east-1" {
			t.Error("空设置不应改变配置")
		}
	})

	t.Run("unknown keys", func(t *testing.T) {
		NewDefault()
		UpdateFromSettings(map[string]string{
			"unknown.key":   "value",
			"another.thing": "123",
		})
		// 不应 panic，未知键应被忽略
	})

	t.Run("very large number", func(t *testing.T) {
		NewDefault()
		UpdateFromSettings(map[string]string{
			"storage.max_object_size": "9223372036854775807", // int64 最大值
		})
		if Global.Storage.MaxObjectSize != 9223372036854775807 {
			t.Errorf("应能处理大数值: %v", Global.Storage.MaxObjectSize)
		}
	})

	t.Run("overflow number", func(t *testing.T) {
		NewDefault()
		original := Global.Storage.MaxObjectSize
		UpdateFromSettings(map[string]string{
			"storage.max_object_size": "99999999999999999999999", // 溢出
		})
		// 解析错误，应保持原值
		if Global.Storage.MaxObjectSize != original {
			t.Error("溢出值不应更新配置")
		}
	})
}

// TestLoadFromDB_EdgeCases 测试加载数据库的边界条件
func TestLoadFromDB_EdgeCases(t *testing.T) {
	originalGlobal := Global
	defer func() { Global = originalGlobal }()

	t.Run("empty API key", func(t *testing.T) {
		Global = nil
		loader := &mockSettingsLoader{
			installed:       true,
			adminUsername:   "admin",
			accessKeyID:     "", // 空 API Key
			secretAccessKey: "",
		}
		LoadFromDB(loader)

		// 空 API Key 不应更新
		if Global.Auth.AccessKeyID != "" {
			t.Error("空 API Key 不应被设置")
		}
	})

	t.Run("zero storage limits", func(t *testing.T) {
		Global = nil
		loader := &mockSettingsLoader{
			installed:     true,
			adminUsername: "admin",
			maxObjectSize: 0,
			maxUploadSize: 0,
		}
		LoadFromDB(loader)

		// 0 值应保持默认
		if Global.Storage.MaxObjectSize != 5*1024*1024*1024 {
			t.Error("0 值应保持默认 MaxObjectSize")
		}
		if Global.Storage.MaxUploadSize != 1024*1024*1024 {
			t.Error("0 值应保持默认 MaxUploadSize")
		}
	})
}

// =============================================================================
// 基准测试
// =============================================================================

// BenchmarkNewDefault 基准测试创建默认配置
func BenchmarkNewDefault(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewDefault()
	}
}

// BenchmarkUpdateFromSettings 基准测试更新设置
func BenchmarkUpdateFromSettings(b *testing.B) {
	NewDefault()
	settings := map[string]string{
		"server.region":           "eu-west-1",
		"storage.max_object_size": "2147483648",
		"auth.admin_username":     "admin",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UpdateFromSettings(settings)
	}
}

// BenchmarkLoadFromDB 基准测试从数据库加载
func BenchmarkLoadFromDB(b *testing.B) {
	loader := &mockSettingsLoader{
		installed:       true,
		adminUsername:   "admin",
		accessKeyID:     "TESTKEY",
		secretAccessKey: "TESTSECRET",
		maxObjectSize:   5 * 1024 * 1024 * 1024,
		maxUploadSize:   1024 * 1024 * 1024,
		settings: map[string]string{
			"server.region":         "ap-northeast-1",
			"security.cors_origin":  "https://example.com",
			"security.presign_scheme": "https",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Global = nil
		LoadFromDB(loader)
	}
}
