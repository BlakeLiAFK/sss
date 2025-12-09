package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	appconfig "sss/internal/config"
	"sss/internal/auth"
	"sss/internal/storage"
	"sss/internal/utils"
)

// 设置基准测试环境
func setupBenchmark(b *testing.B) (*Server, func()) {
	b.Helper()
	utils.InitLogger("error") // 关闭日志减少干扰

	tmpDir, _ := os.MkdirTemp("", "sss-bench-*")
	metadata, _ := storage.NewMetadataStore(tmpDir + "/metadata.db")
	filestore, _ := storage.NewFileStore(tmpDir + "/data")

	appconfig.Global = &appconfig.Config{
		Auth: appconfig.AuthConfig{
			AccessKeyID:     "BENCHACCESSKEY12345678",
			SecretAccessKey: "BENCHSECRETKEY1234567890ABCDEFGHIJ",
		},
		Server: appconfig.ServerConfig{
			Host:   "localhost",
			Port:   8080,
			Region: "us-east-1",
		},
	}
	auth.InitAPIKeyCache(metadata)

	server := NewServer(metadata, filestore)

	// 创建测试桶和对象
	metadata.CreateBucket("bench-bucket")
	metadata.UpdateBucketPublic("bench-bucket", true)
	storagePath, _, _ := filestore.PutObject("bench-bucket", "test.txt", strings.NewReader("benchmark content"), 17)
	metadata.PutObject(&storage.Object{
		Bucket:      "bench-bucket",
		Key:         "test.txt",
		Size:        17,
		ETag:        "test-etag",
		ContentType: "text/plain",
		StoragePath: storagePath,
	})

	cleanup := func() {
		metadata.Close()
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

// BenchmarkPublicBucketGet 测试公开桶 GET 性能
func BenchmarkPublicBucketGet(b *testing.B) {
	server, cleanup := setupBenchmark(b)
	defer cleanup()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/bench-bucket/test.txt", nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkGetBucketOnly 单独测试 GetBucket DB查询
func BenchmarkGetBucketOnly(b *testing.B) {
	utils.InitLogger("error")
	tmpDir, _ := os.MkdirTemp("", "sss-bench-db-*")
	defer os.RemoveAll(tmpDir)

	metadata, _ := storage.NewMetadataStore(tmpDir + "/metadata.db")
	defer metadata.Close()

	metadata.CreateBucket("test-bucket")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metadata.GetBucket("test-bucket")
		}
	})
}

// BenchmarkGetObjectOnly 单独测试 GetObject DB查询
func BenchmarkGetObjectOnly(b *testing.B) {
	utils.InitLogger("error")
	tmpDir, _ := os.MkdirTemp("", "sss-bench-obj-*")
	defer os.RemoveAll(tmpDir)

	metadata, _ := storage.NewMetadataStore(tmpDir + "/metadata.db")
	defer metadata.Close()

	metadata.CreateBucket("test-bucket")
	metadata.PutObject(&storage.Object{
		Bucket:      "test-bucket",
		Key:         "test.txt",
		Size:        100,
		ETag:        "test",
		StoragePath: "/tmp/test",
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metadata.GetObject("test-bucket", "test.txt")
		}
	})
}
