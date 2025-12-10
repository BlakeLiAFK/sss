package api

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "sss/internal/config"
	"sss/internal/auth"
	"sss/internal/storage"
	"sss/internal/utils"
)

// TestAPIKeyWithoutPermission 测试新创建的API Key（没有权限）无法进行操作
// 这是用户可能遇到问题的根本原因
func TestAPIKeyWithoutPermission(t *testing.T) {
	utils.InitLogger("warn")

	tmpDir, err := os.MkdirTemp("", "sss-perm-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	metadata, err := storage.NewMetadataStore(tmpDir + "/metadata.db")
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}
	defer metadata.Close()

	filestore, err := storage.NewFileStore(tmpDir + "/data")
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 初始化配置（使用管理员Key）
	adminAccessKey := "ADMIN_ACCESS_KEY_12345"
	adminSecretKey := "ADMIN_SECRET_KEY_1234567890ABCDEFGHIJ"

	appconfig.Global = &appconfig.Config{
		Auth: appconfig.AuthConfig{
			AccessKeyID:     adminAccessKey,
			SecretAccessKey: adminSecretKey,
		},
		Server: appconfig.ServerConfig{
			Host:   "localhost",
			Port:   8080,
			Region: "us-east-1",
		},
	}

	auth.InitAPIKeyCache(metadata)

	server := NewServer(metadata, filestore)
	ts := httptest.NewServer(server)
	defer ts.Close()

	// 1. 使用管理员Key创建bucket
	adminClient, _ := createClientWithCredentials(ts.URL, adminAccessKey, adminSecretKey)
	ctx := context.Background()

	_, err = adminClient.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("perm-test-bucket"),
	})
	if err != nil {
		t.Fatalf("管理员创建bucket失败: %v", err)
	}
	t.Log("✓ 管理员创建bucket成功")

	// 2. 创建新的API Key（没有权限）
	newKey, err := metadata.CreateAPIKey("测试用Key（无权限）")
	if err != nil {
		t.Fatalf("创建API Key失败: %v", err)
	}
	t.Logf("✓ 创建新API Key: %s", newKey.AccessKeyID)

	// 重新加载缓存
	auth.ReloadAPIKeyCache()

	// 3. 使用新Key尝试PUT操作（应该失败）
	newKeyClient, _ := createClientWithCredentials(ts.URL, newKey.AccessKeyID, newKey.SecretAccessKey)

	_, err = newKeyClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("perm-test-bucket"),
		Key:    aws.String("test.txt"),
		Body:   strings.NewReader("test content"),
	})

	if err == nil {
		t.Error("✗ 没有权限的API Key不应该能PUT对象，但成功了！这是一个BUG！")
	} else {
		t.Logf("✓ 没有权限的API Key正确被拒绝: %v", err)
		if !strings.Contains(err.Error(), "AccessDenied") && !strings.Contains(err.Error(), "403") {
			t.Logf("  注意: 错误信息是 %v", err)
		}
	}

	// 4. 给新Key添加读权限（但没有写权限）
	err = metadata.SetAPIKeyPermission(&storage.APIKeyPermission{
		AccessKeyID: newKey.AccessKeyID,
		BucketName:  "perm-test-bucket",
		CanRead:     true,
		CanWrite:    false,
	})
	if err != nil {
		t.Fatalf("设置权限失败: %v", err)
	}
	auth.ReloadAPIKeyCache()
	t.Log("✓ 添加只读权限")

	// 5. 再次尝试PUT（应该仍然失败，因为只有读权限）
	_, err = newKeyClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("perm-test-bucket"),
		Key:    aws.String("test.txt"),
		Body:   strings.NewReader("test content"),
	})

	if err == nil {
		t.Error("✗ 只有读权限的API Key不应该能PUT对象，但成功了！这是一个BUG！")
	} else {
		t.Logf("✓ 只有读权限的API Key正确被拒绝PUT: %v", err)
	}

	// 6. 管理员上传一个对象
	_, err = adminClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("perm-test-bucket"),
		Key:    aws.String("admin-uploaded.txt"),
		Body:   strings.NewReader("admin content"),
	})
	if err != nil {
		t.Fatalf("管理员上传对象失败: %v", err)
	}
	t.Log("✓ 管理员上传对象成功")

	// 7. 使用只读Key尝试GET（应该成功）
	_, err = newKeyClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("perm-test-bucket"),
		Key:    aws.String("admin-uploaded.txt"),
	})
	if err != nil {
		t.Errorf("✗ 有读权限的API Key应该能GET对象，但失败了: %v", err)
	} else {
		t.Log("✓ 有读权限的API Key可以GET对象")
	}

	// 8. 添加写权限
	err = metadata.SetAPIKeyPermission(&storage.APIKeyPermission{
		AccessKeyID: newKey.AccessKeyID,
		BucketName:  "perm-test-bucket",
		CanRead:     true,
		CanWrite:    true,
	})
	if err != nil {
		t.Fatalf("设置权限失败: %v", err)
	}
	auth.ReloadAPIKeyCache()
	t.Log("✓ 添加读写权限")

	// 9. 再次尝试PUT（现在应该成功）
	_, err = newKeyClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("perm-test-bucket"),
		Key:    aws.String("user-uploaded.txt"),
		Body:   strings.NewReader("user content"),
	})

	if err != nil {
		t.Errorf("✗ 有读写权限的API Key应该能PUT对象，但失败了: %v", err)
	} else {
		t.Log("✓ 有读写权限的API Key可以PUT对象")
	}

	t.Log("\n=== 测试总结 ===")
	t.Log("新创建的API Key默认没有任何权限，必须手动分配权限后才能使用！")
	t.Log("这可能是用户遇到问题的原因。")
}

// TestAPIKeyWithWildcardPermission 测试使用通配符(*)权限的API Key
func TestAPIKeyWithWildcardPermission(t *testing.T) {
	utils.InitLogger("warn")

	tmpDir, err := os.MkdirTemp("", "sss-wildcard-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	metadata, err := storage.NewMetadataStore(tmpDir + "/metadata.db")
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}
	defer metadata.Close()

	filestore, err := storage.NewFileStore(tmpDir + "/data")
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	adminAccessKey := "ADMIN_KEY_123456789012"
	adminSecretKey := "ADMIN_SECRET_1234567890ABCDEFGHIJKLMNO"

	appconfig.Global = &appconfig.Config{
		Auth: appconfig.AuthConfig{
			AccessKeyID:     adminAccessKey,
			SecretAccessKey: adminSecretKey,
		},
		Server: appconfig.ServerConfig{
			Host:   "localhost",
			Port:   8080,
			Region: "us-east-1",
		},
	}

	auth.InitAPIKeyCache(metadata)

	server := NewServer(metadata, filestore)
	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx := context.Background()

	// 1. 管理员创建bucket
	adminClient, _ := createClientWithCredentials(ts.URL, adminAccessKey, adminSecretKey)
	_, err = adminClient.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("wildcard-test-bucket"),
	})
	if err != nil {
		t.Fatalf("创建bucket失败: %v", err)
	}

	// 2. 创建新API Key并设置通配符权限
	newKey, err := metadata.CreateAPIKey("通配符权限Key")
	if err != nil {
		t.Fatalf("创建API Key失败: %v", err)
	}

	// 设置通配符权限 (*)
	err = metadata.SetAPIKeyPermission(&storage.APIKeyPermission{
		AccessKeyID: newKey.AccessKeyID,
		BucketName:  "*", // 通配符
		CanRead:     true,
		CanWrite:    true,
	})
	if err != nil {
		t.Fatalf("设置通配符权限失败: %v", err)
	}
	auth.ReloadAPIKeyCache()
	t.Log("✓ 创建了带通配符(*)权限的API Key")

	// 3. 使用通配符Key进行操作
	wildcardClient, _ := createClientWithCredentials(ts.URL, newKey.AccessKeyID, newKey.SecretAccessKey)

	// PUT操作
	_, err = wildcardClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("wildcard-test-bucket"),
		Key:    aws.String("wildcard-test.txt"),
		Body:   strings.NewReader("wildcard content"),
	})
	if err != nil {
		t.Errorf("✗ 通配符权限Key应该能PUT: %v", err)
	} else {
		t.Log("✓ 通配符权限Key可以PUT对象")
	}

	// GET操作
	_, err = wildcardClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("wildcard-test-bucket"),
		Key:    aws.String("wildcard-test.txt"),
	})
	if err != nil {
		t.Errorf("✗ 通配符权限Key应该能GET: %v", err)
	} else {
		t.Log("✓ 通配符权限Key可以GET对象")
	}

	// DELETE操作
	_, err = wildcardClient.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String("wildcard-test-bucket"),
		Key:    aws.String("wildcard-test.txt"),
	})
	if err != nil {
		t.Errorf("✗ 通配符权限Key应该能DELETE: %v", err)
	} else {
		t.Log("✓ 通配符权限Key可以DELETE对象")
	}
}

// TestDisabledAPIKey 测试禁用的API Key无法使用
func TestDisabledAPIKey(t *testing.T) {
	utils.InitLogger("warn")

	tmpDir, err := os.MkdirTemp("", "sss-disabled-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	metadata, err := storage.NewMetadataStore(tmpDir + "/metadata.db")
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}
	defer metadata.Close()

	filestore, err := storage.NewFileStore(tmpDir + "/data")
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	adminAccessKey := "ADMIN_KEY_DISABLED_TEST"
	adminSecretKey := "ADMIN_SECRET_DISABLED_1234567890ABCDEF"

	appconfig.Global = &appconfig.Config{
		Auth: appconfig.AuthConfig{
			AccessKeyID:     adminAccessKey,
			SecretAccessKey: adminSecretKey,
		},
		Server: appconfig.ServerConfig{
			Host:   "localhost",
			Port:   8080,
			Region: "us-east-1",
		},
	}

	auth.InitAPIKeyCache(metadata)

	server := NewServer(metadata, filestore)
	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx := context.Background()

	// 1. 创建bucket
	adminClient, _ := createClientWithCredentials(ts.URL, adminAccessKey, adminSecretKey)
	_, err = adminClient.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("disabled-test-bucket"),
	})
	if err != nil {
		t.Fatalf("创建bucket失败: %v", err)
	}

	// 2. 创建API Key，设置权限，然后禁用
	newKey, _ := metadata.CreateAPIKey("将被禁用的Key")
	metadata.SetAPIKeyPermission(&storage.APIKeyPermission{
		AccessKeyID: newKey.AccessKeyID,
		BucketName:  "*",
		CanRead:     true,
		CanWrite:    true,
	})
	auth.ReloadAPIKeyCache()

	// 先验证Key可以工作
	keyClient, _ := createClientWithCredentials(ts.URL, newKey.AccessKeyID, newKey.SecretAccessKey)
	_, err = keyClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("disabled-test-bucket"),
		Key:    aws.String("before-disable.txt"),
		Body:   strings.NewReader("before disable"),
	})
	if err != nil {
		t.Fatalf("启用状态下应该能工作: %v", err)
	}
	t.Log("✓ API Key启用时可以正常工作")

	// 3. 禁用API Key
	err = metadata.UpdateAPIKeyEnabled(newKey.AccessKeyID, false)
	if err != nil {
		t.Fatalf("禁用API Key失败: %v", err)
	}
	auth.ReloadAPIKeyCache()
	t.Log("✓ 已禁用API Key")

	// 4. 验证禁用后无法使用
	_, err = keyClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("disabled-test-bucket"),
		Key:    aws.String("after-disable.txt"),
		Body:   strings.NewReader("after disable"),
	})
	if err == nil {
		t.Error("✗ 禁用的API Key不应该能工作！这是一个BUG！")
	} else {
		t.Logf("✓ 禁用的API Key正确被拒绝: %v", err)
	}
}

// createClientWithCredentials 创建带指定凭证的S3客户端
func createClientWithCredentials(endpoint, accessKey, secretKey string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	}), nil
}
