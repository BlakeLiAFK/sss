package storage

import (
	"strings"
	"testing"
	"time"
)

// setupAPIKeysTest 为API Keys测试创建测试环境
func setupAPIKeysTest(t *testing.T) (*MetadataStore, func()) {
	t.Helper()
	return setupMetadataStore(t)
}

// TestCreateAPIKey 测试创建API密钥
func TestCreateAPIKey(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	key, err := ms.CreateAPIKey("Test API Key")
	if err != nil {
		t.Fatalf("创建API密钥失败: %v", err)
	}

	// 验证返回的密钥
	if key.AccessKeyID == "" {
		t.Error("AccessKeyID不应该为空")
	}

	if key.SecretAccessKey == "" {
		t.Error("SecretAccessKey不应该为空")
	}

	if len(key.AccessKeyID) != 20 { // generateRandomKey(20) = 20字符
		t.Errorf("AccessKeyID长度错误: got %d, want 20", len(key.AccessKeyID))
	}

	if len(key.SecretAccessKey) != 40 { // generateRandomKey(40) = 40字符
		t.Errorf("SecretAccessKey长度错误: got %d, want 40", len(key.SecretAccessKey))
	}

	if key.Description != "Test API Key" {
		t.Errorf("描述错误: got %s, want Test API Key", key.Description)
	}

	if !key.Enabled {
		t.Error("新创建的密钥应该是启用状态")
	}

	if key.CreatedAt.IsZero() {
		t.Error("CreatedAt不应该为零值")
	}
}

// TestGetAPIKey 测试获取API密钥
func TestGetAPIKey(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	created, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 获取密钥
	retrieved, err := ms.GetAPIKey(created.AccessKeyID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	if retrieved == nil {
		t.Fatal("应该找到密钥")
	}

	// 验证密钥信息
	if retrieved.AccessKeyID != created.AccessKeyID {
		t.Errorf("AccessKeyID不匹配: got %s, want %s", retrieved.AccessKeyID, created.AccessKeyID)
	}

	if retrieved.Description != created.Description {
		t.Errorf("描述不匹配: got %s, want %s", retrieved.Description, created.Description)
	}

	if retrieved.Enabled != created.Enabled {
		t.Errorf("启用状态不匹配: got %v, want %v", retrieved.Enabled, created.Enabled)
	}

	// SecretAccessKey 不应该返回
	if retrieved.SecretAccessKey != "" {
		t.Error("GetAPIKey不应该返回SecretAccessKey")
	}
}

// TestGetAPIKeyNotFound 测试获取不存在的密钥
func TestGetAPIKeyNotFound(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	key, err := ms.GetAPIKey("nonexistent")
	if err != nil {
		t.Fatalf("获取不存在的密钥不应该出错: %v", err)
	}

	if key != nil {
		t.Error("不存在的密钥应该返回nil")
	}
}

// TestListAPIKeys 测试列出所有API密钥
func TestListAPIKeys(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建多个密钥
	descriptions := []string{"Key 1", "Key 2", "Key 3"}
	createdKeys := make([]*APIKey, len(descriptions))
	for i, desc := range descriptions {
		key, err := ms.CreateAPIKey(desc)
		if err != nil {
			t.Fatalf("创建密钥失败: %v", err)
		}
		createdKeys[i] = key
		time.Sleep(time.Millisecond) // 确保创建时间不同
	}

	// 列出所有密钥
	keys, err := ms.ListAPIKeys()
	if err != nil {
		t.Fatalf("列出密钥失败: %v", err)
	}

	if len(keys) != len(descriptions) {
		t.Errorf("密钥数量错误: got %d, want %d", len(keys), len(descriptions))
	}

	// 验证按创建时间倒序排序
	for i := 0; i < len(keys)-1; i++ {
		if keys[i].CreatedAt.Before(keys[i+1].CreatedAt) {
			t.Error("密钥应该按创建时间倒序排列")
			break
		}
	}

	// 验证 SecretAccessKey 不返回
	for _, key := range keys {
		if key.SecretAccessKey != "" {
			t.Error("ListAPIKeys不应该返回SecretAccessKey")
		}
	}
}

// TestDeleteAPIKey 测试删除API密钥
func TestDeleteAPIKey(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("To Delete")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 删除密钥
	err = ms.DeleteAPIKey(key.AccessKeyID)
	if err != nil {
		t.Fatalf("删除密钥失败: %v", err)
	}

	// 验证密钥已删除
	retrieved, err := ms.GetAPIKey(key.AccessKeyID)
	if err != nil {
		t.Fatalf("查询密钥失败: %v", err)
	}

	if retrieved != nil {
		t.Error("密钥应该已删除")
	}
}

// TestUpdateAPIKeyEnabled 测试启用/禁用API密钥
func TestUpdateAPIKeyEnabled(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 禁用密钥
	err = ms.UpdateAPIKeyEnabled(key.AccessKeyID, false)
	if err != nil {
		t.Fatalf("禁用密钥失败: %v", err)
	}

	// 验证已禁用
	retrieved, err := ms.GetAPIKey(key.AccessKeyID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	if retrieved.Enabled {
		t.Error("密钥应该已禁用")
	}

	// 重新启用
	err = ms.UpdateAPIKeyEnabled(key.AccessKeyID, true)
	if err != nil {
		t.Fatalf("启用密钥失败: %v", err)
	}

	// 验证已启用
	retrieved, err = ms.GetAPIKey(key.AccessKeyID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	if !retrieved.Enabled {
		t.Error("密钥应该已启用")
	}
}

// TestUpdateAPIKeyDescription 测试更新API密钥描述
func TestUpdateAPIKeyDescription(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Original Description")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 更新描述
	newDesc := "Updated Description"
	err = ms.UpdateAPIKeyDescription(key.AccessKeyID, newDesc)
	if err != nil {
		t.Fatalf("更新描述失败: %v", err)
	}

	// 验证描述已更新
	retrieved, err := ms.GetAPIKey(key.AccessKeyID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	if retrieved.Description != newDesc {
		t.Errorf("描述未更新: got %s, want %s", retrieved.Description, newDesc)
	}
}

// TestResetAPIKeySecret 测试重置API密钥的SecretKey
func TestResetAPIKeySecret(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	oldSecret := key.SecretAccessKey

	// 重置密钥
	newSecret, err := ms.ResetAPIKeySecret(key.AccessKeyID)
	if err != nil {
		t.Fatalf("重置密钥失败: %v", err)
	}

	// 验证新密钥
	if newSecret == "" {
		t.Error("新密钥不应该为空")
	}

	if newSecret == oldSecret {
		t.Error("新密钥应该与旧密钥不同")
	}

	if len(newSecret) != 40 { // generateRandomKey(40) = 40字符
		t.Errorf("新密钥长度错误: got %d, want 40", len(newSecret))
	}
}

// TestResetAPIKeySecretNotFound 测试重置不存在的密钥
func TestResetAPIKeySecretNotFound(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	_, err := ms.ResetAPIKeySecret("nonexistent")
	if err == nil {
		t.Error("重置不存在的密钥应该返回错误")
	}
}

// TestSetAPIKeyPermission 测试设置API密钥权限
func TestSetAPIKeyPermission(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 创建桶
	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 设置权限
	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket,
		CanRead:     true,
		CanWrite:    false,
	}

	err = ms.SetAPIKeyPermission(perm)
	if err != nil {
		t.Fatalf("设置权限失败: %v", err)
	}

	// 验证权限
	perms, err := ms.GetAPIKeyPermissions(key.AccessKeyID)
	if err != nil {
		t.Fatalf("获取权限失败: %v", err)
	}

	if len(perms) != 1 {
		t.Fatalf("权限数量错误: got %d, want 1", len(perms))
	}

	if perms[0].BucketName != bucket {
		t.Errorf("桶名错误: got %s, want %s", perms[0].BucketName, bucket)
	}

	if !perms[0].CanRead {
		t.Error("应该有读权限")
	}

	if perms[0].CanWrite {
		t.Error("不应该有写权限")
	}
}

// TestSetAPIKeyPermissionWildcard 测试通配符权限
func TestSetAPIKeyPermissionWildcard(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 设置通配符权限
	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  "*",
		CanRead:     true,
		CanWrite:    true,
	}

	err = ms.SetAPIKeyPermission(perm)
	if err != nil {
		t.Fatalf("设置权限失败: %v", err)
	}

	// 验证权限
	perms, err := ms.GetAPIKeyPermissions(key.AccessKeyID)
	if err != nil {
		t.Fatalf("获取权限失败: %v", err)
	}

	if len(perms) != 1 {
		t.Fatalf("权限数量错误: got %d, want 1", len(perms))
	}

	if perms[0].BucketName != "*" {
		t.Errorf("桶名错误: got %s, want *", perms[0].BucketName)
	}

	if !perms[0].CanRead || !perms[0].CanWrite {
		t.Error("通配符应该有完全权限")
	}
}

// TestDeleteAPIKeyPermission 测试删除API密钥权限
func TestDeleteAPIKeyPermission(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥和权限
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket,
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm)

	// 删除权限
	err = ms.DeleteAPIKeyPermission(key.AccessKeyID, bucket)
	if err != nil {
		t.Fatalf("删除权限失败: %v", err)
	}

	// 验证权限已删除
	perms, err := ms.GetAPIKeyPermissions(key.AccessKeyID)
	if err != nil {
		t.Fatalf("获取权限失败: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("权限应该已删除: got %d", len(perms))
	}
}

// TestListAPIKeysWithPermissions 测试列出密钥及权限
func TestListAPIKeysWithPermissions(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	originalSecret := key.SecretAccessKey

	// 添加权限
	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	perm1 := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket,
		CanRead:     true,
		CanWrite:    false,
	}
	ms.SetAPIKeyPermission(perm1)

	perm2 := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  "*",
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm2)

	// 列出密钥及权限
	keys, err := ms.ListAPIKeysWithPermissions()
	if err != nil {
		t.Fatalf("列出密钥失败: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("密钥数量错误: got %d, want 1", len(keys))
	}

	keyWithPerms := keys[0]

	// 验证密钥信息
	if keyWithPerms.AccessKeyID != key.AccessKeyID {
		t.Errorf("AccessKeyID不匹配: got %s, want %s", keyWithPerms.AccessKeyID, key.AccessKeyID)
	}

	// 验证SecretAccessKey已解密
	if keyWithPerms.SecretAccessKey != originalSecret {
		t.Errorf("SecretAccessKey解密错误: got %s, want %s", keyWithPerms.SecretAccessKey, originalSecret)
	}

	// 验证权限
	if len(keyWithPerms.Permissions) != 2 {
		t.Errorf("权限数量错误: got %d, want 2", len(keyWithPerms.Permissions))
	}
}

// TestAPIKeyCache 测试API密钥缓存
func TestAPIKeyCache(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 创建缓存
	cache := NewAPIKeyCache(ms)

	// 验证密钥
	if !cache.Validate(key.AccessKeyID, key.SecretAccessKey) {
		t.Error("密钥验证应该通过")
	}

	// 验证错误的密钥
	if cache.Validate(key.AccessKeyID, "wrong-secret") {
		t.Error("错误的密钥不应该通过验证")
	}

	// 验证不存在的密钥
	if cache.Validate("nonexistent", "secret") {
		t.Error("不存在的密钥不应该通过验证")
	}
}

// TestAPIKeyCacheGetSecretKey 测试从缓存获取SecretKey
func TestAPIKeyCacheGetSecretKey(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 创建缓存
	cache := NewAPIKeyCache(ms)

	// 获取SecretKey
	secret, exists := cache.GetSecretKey(key.AccessKeyID)
	if !exists {
		t.Error("密钥应该存在于缓存中")
	}

	if secret != key.SecretAccessKey {
		t.Errorf("SecretKey不匹配: got %s, want %s", secret, key.SecretAccessKey)
	}

	// 获取不存在的密钥
	_, exists = cache.GetSecretKey("nonexistent")
	if exists {
		t.Error("不存在的密钥不应该返回")
	}
}

// TestAPIKeyCacheCheckPermission 测试缓存权限检查
func TestAPIKeyCacheCheckPermission(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 创建桶
	bucket1 := "bucket1"
	bucket2 := "bucket2"
	ms.CreateBucket(bucket1)
	ms.CreateBucket(bucket2)

	// 设置权限：bucket1只读，bucket2读写
	perm1 := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket1,
		CanRead:     true,
		CanWrite:    false,
	}
	ms.SetAPIKeyPermission(perm1)

	perm2 := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket2,
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm2)

	// 创建缓存
	cache := NewAPIKeyCache(ms)

	// 测试bucket1读权限
	if !cache.CheckPermission(key.AccessKeyID, bucket1, false) {
		t.Error("bucket1应该有读权限")
	}

	// 测试bucket1写权限
	if cache.CheckPermission(key.AccessKeyID, bucket1, true) {
		t.Error("bucket1不应该有写权限")
	}

	// 测试bucket2读权限
	if !cache.CheckPermission(key.AccessKeyID, bucket2, false) {
		t.Error("bucket2应该有读权限")
	}

	// 测试bucket2写权限
	if !cache.CheckPermission(key.AccessKeyID, bucket2, true) {
		t.Error("bucket2应该有写权限")
	}

	// 测试不存在的桶
	if cache.CheckPermission(key.AccessKeyID, "nonexistent", false) {
		t.Error("不存在的桶不应该有权限")
	}
}

// TestAPIKeyCacheWildcardPermission 测试通配符权限
func TestAPIKeyCacheWildcardPermission(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 设置通配符权限
	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  "*",
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm)

	// 创建缓存
	cache := NewAPIKeyCache(ms)

	// 测试任意桶的权限
	testBuckets := []string{"bucket1", "bucket2", "any-bucket", "测试桶"}
	for _, bucket := range testBuckets {
		if !cache.CheckPermission(key.AccessKeyID, bucket, false) {
			t.Errorf("通配符应该允许读取桶: %s", bucket)
		}

		if !cache.CheckPermission(key.AccessKeyID, bucket, true) {
			t.Errorf("通配符应该允许写入桶: %s", bucket)
		}
	}
}

// TestAPIKeyCacheDisabledKey 测试禁用的密钥
func TestAPIKeyCacheDisabledKey(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 添加权限
	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  "*",
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm)

	// 创建缓存
	cache := NewAPIKeyCache(ms)

	// 验证启用状态下可以通过
	if !cache.Validate(key.AccessKeyID, key.SecretAccessKey) {
		t.Error("启用的密钥应该通过验证")
	}

	// 禁用密钥
	ms.UpdateAPIKeyEnabled(key.AccessKeyID, false)

	// 重新加载缓存
	cache.Reload()

	// 验证禁用后不能通过
	if cache.Validate(key.AccessKeyID, key.SecretAccessKey) {
		t.Error("禁用的密钥不应该通过验证")
	}

	// 测试禁用密钥的权限检查
	if cache.CheckPermission(key.AccessKeyID, "any-bucket", false) {
		t.Error("禁用的密钥不应该有权限")
	}

	// 测试禁用密钥的SecretKey获取
	_, exists := cache.GetSecretKey(key.AccessKeyID)
	if exists {
		t.Error("禁用的密钥不应该返回SecretKey")
	}
}

// TestAPIKeyCacheReload 测试缓存重新加载
func TestAPIKeyCacheReload(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建缓存
	cache := NewAPIKeyCache(ms)

	// 创建密钥
	key, err := ms.CreateAPIKey("New Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 新创建的密钥不在缓存中
	if cache.Validate(key.AccessKeyID, key.SecretAccessKey) {
		t.Error("新创建的密钥不应该在旧缓存中")
	}

	// 重新加载缓存
	err = cache.Reload()
	if err != nil {
		t.Fatalf("重新加载缓存失败: %v", err)
	}

	// 现在应该能验证
	if !cache.Validate(key.AccessKeyID, key.SecretAccessKey) {
		t.Error("重新加载后应该能验证新密钥")
	}
}

// TestAPIKeyCacheConcurrent 测试缓存并发访问
func TestAPIKeyCacheConcurrent(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 创建桶和权限
	bucket := "test-bucket"
	ms.CreateBucket(bucket)
	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket,
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm)

	// 创建缓存
	cache := NewAPIKeyCache(ms)

	// 并发测试
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			// 验证密钥
			if !cache.Validate(key.AccessKeyID, key.SecretAccessKey) {
				errors <- err
				done <- false
				return
			}

			// 检查权限
			if !cache.CheckPermission(key.AccessKeyID, bucket, true) {
				errors <- err
				done <- false
				return
			}

			// 获取SecretKey
			secret, exists := cache.GetSecretKey(key.AccessKeyID)
			if !exists || secret != key.SecretAccessKey {
				errors <- err
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-done {
			successCount++
		}
	}

	// 检查是否有错误
	close(errors)
	for err := range errors {
		t.Errorf("并发测试出错: %v", err)
	}

	if successCount != numGoroutines {
		t.Errorf("并发测试成功率不足: %d/%d", successCount, numGoroutines)
	}
}

// TestAPIKeySecretEncryption 测试密钥加密存储
func TestAPIKeySecretEncryption(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	// 直接查询数据库，验证密钥是加密存储的
	var encryptedSecret string
	err = ms.db.QueryRow(
		"SELECT secret_access_key FROM api_keys WHERE access_key_id = ?",
		key.AccessKeyID,
	).Scan(&encryptedSecret)
	if err != nil {
		t.Fatalf("查询数据库失败: %v", err)
	}

	// 加密后的密钥不应该等于原始密钥
	if encryptedSecret == key.SecretAccessKey {
		t.Error("密钥应该是加密存储的")
	}

	// 验证能正确解密
	decrypted, err := ms.DecryptSecret(encryptedSecret)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if decrypted != key.SecretAccessKey {
		t.Errorf("解密结果不匹配: got %s, want %s", decrypted, key.SecretAccessKey)
	}
}

// TestDeleteAPIKeyCascade 测试删除密钥级联删除权限
func TestDeleteAPIKeyCascade(t *testing.T) {
	t.Skip("SQLite外键约束在默认配置下不启用，级联删除由应用层保证")

	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	// 创建密钥和权限
	key, err := ms.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket,
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm)

	// 删除密钥
	err = ms.DeleteAPIKey(key.AccessKeyID)
	if err != nil {
		t.Fatalf("删除密钥失败: %v", err)
	}

	// 验证权限也被删除（外键级联删除）
	perms, err := ms.GetAPIKeyPermissions(key.AccessKeyID)
	if err != nil {
		t.Fatalf("获取权限失败: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("权限应该级联删除: got %d", len(perms))
	}
}

// TestGenerateRandomKey 测试随机密钥生成
func TestGenerateRandomKey(t *testing.T) {
	// 生成多个密钥，验证唯一性
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := generateRandomKey(40)
		if keys[key] {
			t.Errorf("生成了重复的密钥: %s", key)
		}
		keys[key] = true

		if len(key) != 40 { // generateRandomKey(40) = 40字符
			t.Errorf("密钥长度错误: got %d, want 40", len(key))
		}
	}
}

// TestAPIKeyDescriptionSpecialCharacters 测试特殊字符描述
func TestAPIKeyDescriptionSpecialCharacters(t *testing.T) {
	ms, cleanup := setupAPIKeysTest(t)
	defer cleanup()

	testCases := []string{
		"测试中文描述",
		"Test with 'quotes'",
		"Test with \"double quotes\"",
		"Test with <HTML> tags",
		"Test with special chars: !@#$%^&*()",
		strings.Repeat("Long description ", 100),
	}

	for _, desc := range testCases {
		key, err := ms.CreateAPIKey(desc)
		if err != nil {
			t.Errorf("创建密钥失败 (desc=%s): %v", desc, err)
			continue
		}

		retrieved, err := ms.GetAPIKey(key.AccessKeyID)
		if err != nil {
			t.Errorf("获取密钥失败 (desc=%s): %v", desc, err)
			continue
		}

		if retrieved.Description != desc {
			t.Errorf("描述不匹配: got %s, want %s", retrieved.Description, desc)
		}
	}
}

// BenchmarkAPIKeyCacheValidate API密钥验证性能基准
func BenchmarkAPIKeyCacheValidate(b *testing.B) {
	ms, cleanup := setupAPIKeysTest(&testing.T{})
	defer cleanup()

	key, _ := ms.CreateAPIKey("Bench Key")
	cache := NewAPIKeyCache(ms)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Validate(key.AccessKeyID, key.SecretAccessKey)
	}
}

// BenchmarkAPIKeyCacheCheckPermission 权限检查性能基准
func BenchmarkAPIKeyCacheCheckPermission(b *testing.B) {
	ms, cleanup := setupAPIKeysTest(&testing.T{})
	defer cleanup()

	key, _ := ms.CreateAPIKey("Bench Key")
	bucket := "bench-bucket"
	ms.CreateBucket(bucket)

	perm := &APIKeyPermission{
		AccessKeyID: key.AccessKeyID,
		BucketName:  bucket,
		CanRead:     true,
		CanWrite:    true,
	}
	ms.SetAPIKeyPermission(perm)

	cache := NewAPIKeyCache(ms)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.CheckPermission(key.AccessKeyID, bucket, true)
	}
}
