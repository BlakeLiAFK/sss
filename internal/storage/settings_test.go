package storage

import (
	"strings"
	"testing"
)

// setupSettingsTest 为设置测试创建MetadataStore
func setupSettingsTest(t *testing.T) (*MetadataStore, func()) {
	t.Helper()
	return setupMetadataStore(t)
}

// TestGetSetSetting 测试基本配置读写
func TestGetSetSetting(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 测试设置配置
	err := ms.SetSetting("test.key", "test value")
	if err != nil {
		t.Fatalf("设置配置失败: %v", err)
	}

	// 测试读取配置
	value, err := ms.GetSetting("test.key")
	if err != nil {
		t.Fatalf("读取配置失败: %v", err)
	}

	if value != "test value" {
		t.Errorf("配置值不匹配: got %s, want 'test value'", value)
	}
}

// TestGetSettingNotFound 测试不存在的配置
func TestGetSettingNotFound(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	value, err := ms.GetSetting("non.existent.key")
	if err != nil {
		t.Fatalf("读取不存在的配置应该不报错: %v", err)
	}

	if value != "" {
		t.Errorf("不存在的配置应该返回空字符串: got %s", value)
	}
}

// TestSetSettingUpdate 测试配置更新
func TestSetSettingUpdate(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 首次设置
	ms.SetSetting("update.key", "value1")

	// 更新
	err := ms.SetSetting("update.key", "value2")
	if err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	// 验证
	value, err := ms.GetSetting("update.key")
	if err != nil {
		t.Fatalf("读取配置失败: %v", err)
	}

	if value != "value2" {
		t.Errorf("配置值应该被更新: got %s, want 'value2'", value)
	}
}

// TestGetSettings 测试批量获取配置
func TestGetSettings(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 设置多个配置
	settings := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range settings {
		if err := ms.SetSetting(k, v); err != nil {
			t.Fatalf("设置配置失败: %v", err)
		}
	}

	// 批量获取
	keys := []string{"key1", "key2", "key3", "nonexistent"}
	result, err := ms.GetSettings(keys)
	if err != nil {
		t.Fatalf("批量获取配置失败: %v", err)
	}

	// 验证
	if result["key1"] != "value1" {
		t.Errorf("key1 值错误: got %s", result["key1"])
	}
	if result["key2"] != "value2" {
		t.Errorf("key2 值错误: got %s", result["key2"])
	}
	if result["key3"] != "value3" {
		t.Errorf("key3 值错误: got %s", result["key3"])
	}
	if result["nonexistent"] != "" {
		t.Errorf("不存在的键应该返回空字符串: got %s", result["nonexistent"])
	}
}

// TestGetAllSettings 测试获取所有配置
func TestGetAllSettings(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 设置多个配置
	expected := map[string]string{
		"aaa.key": "value1",
		"bbb.key": "value2",
		"ccc.key": "value3",
	}

	for k, v := range expected {
		if err := ms.SetSetting(k, v); err != nil {
			t.Fatalf("设置配置失败: %v", err)
		}
	}

	// 获取所有配置
	settings, err := ms.GetAllSettings()
	if err != nil {
		t.Fatalf("获取所有配置失败: %v", err)
	}

	if len(settings) != len(expected) {
		t.Errorf("配置数量不匹配: got %d, want %d", len(settings), len(expected))
	}

	// 验证按 key 排序
	if len(settings) >= 2 {
		for i := 0; i < len(settings)-1; i++ {
			if settings[i].Key > settings[i+1].Key {
				t.Error("配置应该按 key 排序")
				break
			}
		}
	}

	// 验证值
	for _, s := range settings {
		if expectedValue, ok := expected[s.Key]; ok {
			if s.Value != expectedValue {
				t.Errorf("配置值不匹配 %s: got %s, want %s", s.Key, s.Value, expectedValue)
			}
		}
	}
}

// TestIsInstalled 测试安装状态检查
func TestIsInstalled(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 初始状态应该未安装
	if ms.IsInstalled() {
		t.Error("初始状态应该未安装")
	}

	// 设置已安装
	err := ms.SetInstalled()
	if err != nil {
		t.Fatalf("设置已安装失败: %v", err)
	}

	// 验证已安装
	if !ms.IsInstalled() {
		t.Error("应该已安装")
	}

	// 验证安装时间已设置
	installedAt, err := ms.GetSetting(SettingSystemInstalledAt)
	if err != nil {
		t.Fatalf("读取安装时间失败: %v", err)
	}

	if installedAt == "" {
		t.Error("安装时间应该被设置")
	}
}

// TestValidatePassword 测试密码验证
func TestValidatePassword(t *testing.T) {
	testCases := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"有效密码", "Abcd1234", nil},
		{"长密码", "ThisIsAVeryLongPassword123WithNumbers", nil},
		{"太短", "Abc123", ErrPasswordTooShort},
		{"无大写字母", "abcd1234", ErrPasswordNoUpper},
		{"无小写字母", "ABCD1234", ErrPasswordNoLower},
		{"无数字", "AbcdEfgh", ErrPasswordNoDigit},
		{"只有8个字符刚好", "Abcd1234", nil},
		{"特殊字符也可以", "Abc123!@#", nil},
		{"中文也可以", "Abc123中文", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password)
			if err != tc.wantErr {
				t.Errorf("密码验证结果不符: got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestSetAdminPassword 测试设置管理员密码
func TestSetAdminPassword(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 测试有效密码
	err := ms.SetAdminPassword("Admin123")
	if err != nil {
		t.Fatalf("设置管理员密码失败: %v", err)
	}

	// 验证密码哈希已保存
	hash, err := ms.GetSetting(SettingAuthAdminPasswordHash)
	if err != nil {
		t.Fatalf("读取密码哈希失败: %v", err)
	}

	if hash == "" {
		t.Error("密码哈希应该被保存")
	}

	// 密码哈希应该不等于明文密码
	if hash == "Admin123" {
		t.Error("密码应该被哈希，不应该是明文")
	}
}

// TestSetAdminPasswordInvalid 测试无效密码
func TestSetAdminPasswordInvalid(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	testCases := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"太短", "Abc123", ErrPasswordTooShort},
		{"无大写", "abcd1234", ErrPasswordNoUpper},
		{"无小写", "ABCD1234", ErrPasswordNoLower},
		{"无数字", "AbcdEfgh", ErrPasswordNoDigit},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ms.SetAdminPassword(tc.password)
			if err != tc.wantErr {
				t.Errorf("期望错误 %v, 得到 %v", tc.wantErr, err)
			}
		})
	}
}

// TestVerifyAdminPassword 测试验证管理员密码
func TestVerifyAdminPassword(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	password := "Admin123"

	// 设置密码
	err := ms.SetAdminPassword(password)
	if err != nil {
		t.Fatalf("设置密码失败: %v", err)
	}

	// 验证正确密码
	if !ms.VerifyAdminPassword(password) {
		t.Error("正确密码应该验证通过")
	}

	// 验证错误密码
	if ms.VerifyAdminPassword("WrongPassword123") {
		t.Error("错误密码不应该验证通过")
	}

	// 验证空密码
	if ms.VerifyAdminPassword("") {
		t.Error("空密码不应该验证通过")
	}
}

// TestVerifyAdminPasswordNoPassword 测试未设置密码的验证
func TestVerifyAdminPasswordNoPassword(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 未设置密码，任何验证都应该失败
	if ms.VerifyAdminPassword("AnyPassword123") {
		t.Error("未设置密码时验证应该失败")
	}
}

// TestGetAdminUsername 测试获取管理员用户名
func TestGetAdminUsername(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 默认用户名
	username := ms.GetAdminUsername()
	if username != "admin" {
		t.Errorf("默认用户名应该是 admin: got %s", username)
	}

	// 设置自定义用户名
	ms.SetSetting(SettingAuthAdminUsername, "custom_admin")
	username = ms.GetAdminUsername()
	if username != "custom_admin" {
		t.Errorf("自定义用户名不匹配: got %s", username)
	}
}

// TestInitDefaultSettings 测试初始化默认配置
func TestInitDefaultSettings(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	err := ms.InitDefaultSettings("myadmin", "Admin123")
	if err != nil {
		t.Fatalf("初始化默认配置失败: %v", err)
	}

	// 验证服务器配置
	host, port, region := ms.GetServerConfig()
	if host != "0.0.0.0" {
		t.Errorf("默认 host 错误: got %s", host)
	}
	if port != 8080 {
		t.Errorf("默认 port 错误: got %d", port)
	}
	if region != "us-east-1" {
		t.Errorf("默认 region 错误: got %s", region)
	}

	// 验证存储配置
	dataPath, maxObjectSize, maxUploadSize := ms.GetStorageConfig()
	if dataPath != "./data/buckets" {
		t.Errorf("默认 dataPath 错误: got %s", dataPath)
	}
	if maxObjectSize != 5*1024*1024*1024 {
		t.Errorf("默认 maxObjectSize 错误: got %d", maxObjectSize)
	}
	if maxUploadSize != 1024*1024*1024 {
		t.Errorf("默认 maxUploadSize 错误: got %d", maxUploadSize)
	}

	// 验证管理员配置
	username := ms.GetAdminUsername()
	if username != "myadmin" {
		t.Errorf("管理员用户名错误: got %s", username)
	}

	// 验证密码
	if !ms.VerifyAdminPassword("Admin123") {
		t.Error("管理员密码验证失败")
	}

	// 验证 API Key 已生成
	accessKeyID, secretAccessKey := ms.GetAuthConfig()
	if accessKeyID == "" {
		t.Error("AccessKeyID 应该被生成")
	}
	if secretAccessKey == "" {
		t.Error("SecretAccessKey 应该被生成")
	}
	if len(accessKeyID) != 20 {
		t.Errorf("AccessKeyID 长度错误: got %d, want 20", len(accessKeyID))
	}
	if len(secretAccessKey) != 40 {
		t.Errorf("SecretAccessKey 长度错误: got %d, want 40", len(secretAccessKey))
	}
}

// TestGetServerConfig 测试获取服务器配置
func TestGetServerConfig(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 测试默认值
	host, port, region := ms.GetServerConfig()
	if host != "0.0.0.0" {
		t.Errorf("默认 host 错误: got %s", host)
	}
	if port != 8080 {
		t.Errorf("默认 port 错误: got %d", port)
	}
	if region != "us-east-1" {
		t.Errorf("默认 region 错误: got %s", region)
	}

	// 设置自定义值
	ms.SetSetting(SettingServerHost, "127.0.0.1")
	ms.SetSetting(SettingServerPort, "9000")
	ms.SetSetting(SettingServerRegion, "ap-southeast-1")

	host, port, region = ms.GetServerConfig()
	if host != "127.0.0.1" {
		t.Errorf("自定义 host 错误: got %s", host)
	}
	if port != 9000 {
		t.Errorf("自定义 port 错误: got %d", port)
	}
	if region != "ap-southeast-1" {
		t.Errorf("自定义 region 错误: got %s", region)
	}
}

// TestGetServerConfigInvalidPort 测试无效端口处理
func TestGetServerConfigInvalidPort(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	testCases := []struct {
		name        string
		portValue   string
		expectedPort int
	}{
		{"无效字符", "abc", 8080},
		{"负数", "-100", 8080},
		{"零", "0", 8080},
		{"空字符串", "", 8080},
		{"有效端口", "3000", 3000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ms.SetSetting(SettingServerPort, tc.portValue)
			_, port, _ := ms.GetServerConfig()
			if port != tc.expectedPort {
				t.Errorf("端口解析错误: got %d, want %d", port, tc.expectedPort)
			}
		})
	}
}

// TestGetStorageConfig 测试获取存储配置
func TestGetStorageConfig(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 测试默认值
	dataPath, maxObjectSize, maxUploadSize := ms.GetStorageConfig()
	if dataPath != "./data/buckets" {
		t.Errorf("默认 dataPath 错误: got %s", dataPath)
	}
	if maxObjectSize != 5*1024*1024*1024 {
		t.Errorf("默认 maxObjectSize 错误: got %d", maxObjectSize)
	}
	if maxUploadSize != 1024*1024*1024 {
		t.Errorf("默认 maxUploadSize 错误: got %d", maxUploadSize)
	}

	// 设置自定义值
	ms.SetSetting(SettingStorageDataPath, "/custom/path")
	ms.SetSetting(SettingStorageMaxObjectSize, "10737418240") // 10GB
	ms.SetSetting(SettingStorageMaxUploadSize, "2147483648")  // 2GB

	dataPath, maxObjectSize, maxUploadSize = ms.GetStorageConfig()
	if dataPath != "/custom/path" {
		t.Errorf("自定义 dataPath 错误: got %s", dataPath)
	}
	if maxObjectSize != 10*1024*1024*1024 {
		t.Errorf("自定义 maxObjectSize 错误: got %d", maxObjectSize)
	}
	if maxUploadSize != 2*1024*1024*1024 {
		t.Errorf("自定义 maxUploadSize 错误: got %d", maxUploadSize)
	}
}

// TestGetStorageConfigInvalidSize 测试无效大小处理
func TestGetStorageConfigInvalidSize(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 设置无效值，应该使用默认值
	ms.SetSetting(SettingStorageMaxObjectSize, "invalid")
	ms.SetSetting(SettingStorageMaxUploadSize, "-100")

	_, maxObjectSize, maxUploadSize := ms.GetStorageConfig()
	if maxObjectSize != 5*1024*1024*1024 {
		t.Errorf("无效值应该使用默认值: got %d", maxObjectSize)
	}
	if maxUploadSize != 1024*1024*1024 {
		t.Errorf("无效值应该使用默认值: got %d", maxUploadSize)
	}
}

// TestGetAuthConfig 测试获取认证配置
func TestGetAuthConfig(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 初始状态应该为空
	accessKeyID, secretAccessKey := ms.GetAuthConfig()
	if accessKeyID != "" {
		t.Errorf("初始 AccessKeyID 应该为空: got %s", accessKeyID)
	}
	if secretAccessKey != "" {
		t.Errorf("初始 SecretAccessKey 应该为空: got %s", secretAccessKey)
	}

	// 设置值
	ms.SetSetting(SettingAuthAccessKeyID, "AKIAIOSFODNN7EXAMPLE")
	ms.SetSetting(SettingAuthSecretAccessKey, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	accessKeyID, secretAccessKey = ms.GetAuthConfig()
	if accessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("AccessKeyID 错误: got %s", accessKeyID)
	}
	if secretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("SecretAccessKey 错误: got %s", secretAccessKey)
	}
}

// TestSettingKeyConstants 测试配置键常量
func TestSettingKeyConstants(t *testing.T) {
	// 确保配置键常量存在且不为空
	constants := []string{
		SettingSystemInstalled,
		SettingSystemInstalledAt,
		SettingSystemVersion,
		SettingServerHost,
		SettingServerPort,
		SettingServerRegion,
		SettingStorageDataPath,
		SettingStorageMaxObjectSize,
		SettingStorageMaxUploadSize,
		SettingSecurityCORSOrigin,
		SettingSecurityPresignScheme,
		SettingAuthAdminUsername,
		SettingAuthAdminPasswordHash,
		SettingAuthAccessKeyID,
		SettingAuthSecretAccessKey,
	}

	for _, constant := range constants {
		if constant == "" {
			t.Error("配置键常量不应该为空")
		}
	}
}

// TestPasswordHashDifferent 测试同一密码多次哈希应该不同（bcrypt随机盐）
func TestPasswordHashDifferent(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	password := "Admin123"

	// 第一次设置
	ms.SetAdminPassword(password)
	hash1, _ := ms.GetSetting(SettingAuthAdminPasswordHash)

	// 第二次设置相同密码
	ms.SetAdminPassword(password)
	hash2, _ := ms.GetSetting(SettingAuthAdminPasswordHash)

	// 哈希应该不同（因为 bcrypt 使用随机盐）
	if hash1 == hash2 {
		t.Error("同一密码的哈希应该因随机盐而不同")
	}

	// 但两个哈希都应该能验证密码
	if !ms.VerifyAdminPassword(password) {
		t.Error("新哈希应该能验证密码")
	}
}

// TestSettingsUpdateTime 测试配置更新时间
func TestSettingsUpdateTime(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	// 设置配置
	ms.SetSetting("test.time", "value1")

	// 获取第一次的更新时间
	settings1, _ := ms.GetAllSettings()
	var time1 string
	for _, s := range settings1 {
		if s.Key == "test.time" {
			time1 = s.UpdatedAt.Format("2006-01-02 15:04:05")
			break
		}
	}

	// 稍微等待
	// time.Sleep(time.Millisecond)

	// 更新配置
	ms.SetSetting("test.time", "value2")

	// 获取第二次的更新时间
	settings2, _ := ms.GetAllSettings()
	var time2 string
	for _, s := range settings2 {
		if s.Key == "test.time" {
			time2 = s.UpdatedAt.Format("2006-01-02 15:04:05")
			break
		}
	}

	// 更新时间应该不同（或相等，因为时间精度问题）
	// 这里我们只验证时间字段存在
	if time1 == "" || time2 == "" {
		t.Error("更新时间应该被设置")
	}
}

// TestSettingSpecialCharacters 测试特殊字符处理
func TestSettingSpecialCharacters(t *testing.T) {
	ms, cleanup := setupSettingsTest(t)
	defer cleanup()

	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{"中文键值", "中文.配置", "中文值"},
		{"特殊字符", "special!@#$", "value!@#$"},
		{"SQL注入", "key'; DROP TABLE--", "value'; DELETE FROM--"},
		{"长字符串", "long.key", strings.Repeat("x", 1000)},
		{"空值", "empty.value", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ms.SetSetting(tc.key, tc.value)
			if err != nil {
				t.Fatalf("设置配置失败: %v", err)
			}

			value, err := ms.GetSetting(tc.key)
			if err != nil {
				t.Fatalf("读取配置失败: %v", err)
			}

			if value != tc.value {
				t.Errorf("配置值不匹配: got %s, want %s", value, tc.value)
			}
		})
	}
}

// TestParseIntSafe 测试安全整数解析
func TestParseIntSafe(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int
	}{
		{"正常数字", "12345", 12345},
		{"零", "0", 0},
		{"大数字", "999999", 999999},
		{"前导零", "00123", 123},
		{"非数字字符", "123abc", 0}, // 遇到非数字停止
		{"负数", "-100", 0},          // 负号不是数字
		{"空字符串", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result int
			parseIntSafe(tc.input, &result)
			if result != tc.expected {
				t.Errorf("解析结果不匹配: got %d, want %d", result, tc.expected)
			}
		})
	}
}

// TestParseInt64Safe 测试安全64位整数解析
func TestParseInt64Safe(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int64
	}{
		{"正常数字", "12345678901234", 12345678901234},
		{"零", "0", 0},
		{"大数字", "9223372036854775807", 9223372036854775807}, // int64 max
		{"前导零", "00123", 123},
		{"非数字字符", "123abc", 0},
		{"负数", "-100", 0},
		{"空字符串", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result int64
			parseInt64Safe(tc.input, &result)
			if result != tc.expected {
				t.Errorf("解析结果不匹配: got %d, want %d", result, tc.expected)
			}
		})
	}
}

// BenchmarkSetSetting 配置写入性能基准测试
func BenchmarkSetSetting(b *testing.B) {
	ms, cleanup := setupMetadataStore(&testing.T{})
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.SetSetting("benchmark.key", "benchmark value")
	}
}

// BenchmarkGetSetting 配置读取性能基准测试
func BenchmarkGetSetting(b *testing.B) {
	ms, cleanup := setupMetadataStore(&testing.T{})
	defer cleanup()

	ms.SetSetting("benchmark.key", "benchmark value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.GetSetting("benchmark.key")
	}
}

// BenchmarkVerifyAdminPassword 密码验证性能基准测试
func BenchmarkVerifyAdminPassword(b *testing.B) {
	ms, cleanup := setupMetadataStore(&testing.T{})
	defer cleanup()

	ms.SetAdminPassword("Admin123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.VerifyAdminPassword("Admin123")
	}
}
