package api

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// setupBucketTestServer 创建测试用的服务器
func setupBucketTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	// 初始化配置
	if config.Global == nil {
		config.NewDefault()
	}
	// 初始化日志
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建存储
	metadata, err := storage.NewMetadataStore(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}

	filestore, err := storage.NewFileStore(tempDir)
	if err != nil {
		metadata.Close()
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 创建服务器
	server := NewServer(metadata, filestore)

	cleanup := func() {
		metadata.Close()
	}

	return server, cleanup
}

// createTestBucket 创建测试用的桶
func createTestBucket(t *testing.T, server *Server, bucketName string) {
	t.Helper()

	req := httptest.NewRequest("PUT", "/"+bucketName, nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test-key/20210101/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=test")
	w := httptest.NewRecorder()

	server.handleCreateBucket(w, req, bucketName)

	if w.Code != http.StatusOK {
		t.Fatalf("创建桶失败: %d", w.Code)
	}
}

// TestHandleListBuckets 测试列举存储桶
func TestHandleListBuckets(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	t.Run("空桶列表", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleListBuckets(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusOK)
		}

		// 验证响应内容类型
		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "application/xml") {
			t.Errorf("Content-Type应该是XML: got %s", contentType)
		}

		// 解析响应
		var result ListAllMyBucketsResult
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if len(result.Buckets.Bucket) != 0 {
			t.Errorf("应该没有桶: got %d", len(result.Buckets.Bucket))
		}
	})

	t.Run("有桶列表", func(t *testing.T) {
		// 创建几个测试桶
		createTestBucket(t, server, "bucket1")
		createTestBucket(t, server, "bucket2")
		createTestBucket(t, server, "bucket3")

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleListBuckets(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusOK)
		}

		// 解析响应
		var result ListAllMyBucketsResult
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if len(result.Buckets.Bucket) != 3 {
			t.Errorf("桶数量不正确: got %d, want 3", len(result.Buckets.Bucket))
		}

		// 验证所有者信息
		if result.Owner.ID != config.Global.Auth.AccessKeyID {
			t.Errorf("所有者ID不匹配: got %s", result.Owner.ID)
		}
	})
}

// TestHandleCreateBucket 测试创建存储桶
func TestHandleCreateBucket(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	testCases := []struct {
		name         string
		bucketName   string
		expectedCode int
	}{
		{
			name:         "有效桶名",
			bucketName:   "valid-bucket",
			expectedCode: http.StatusOK,
		},
		{
			name:         "数字开头",
			bucketName:   "123bucket",
			expectedCode: http.StatusOK,
		},
		{
			name:         "包含数字",
			bucketName:   "bucket123",
			expectedCode: http.StatusOK,
		},
		{
			name:         "短名称",
			bucketName:   "abc",
			expectedCode: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", "/"+tc.bucketName, nil)
			w := httptest.NewRecorder()

			server.handleCreateBucket(w, req, tc.bucketName)

			if w.Code != tc.expectedCode {
				body, _ := io.ReadAll(w.Body)
				t.Errorf("状态码不正确: got %d, want %d, body: %s",
					w.Code, tc.expectedCode, string(body))
			}

			if tc.expectedCode == http.StatusOK {
				// 验证Location头
				location := w.Header().Get("Location")
				if location != "/"+tc.bucketName {
					t.Errorf("Location头不正确: got %s, want /%s", location, tc.bucketName)
				}
			}
		})
	}
}

// TestHandleCreateBucketDuplicate 测试创建重复桶
func TestHandleCreateBucketDuplicate(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	bucketName := "test-bucket"

	// 第一次创建应该成功
	req1 := httptest.NewRequest("PUT", "/"+bucketName, nil)
	w1 := httptest.NewRecorder()
	server.handleCreateBucket(w1, req1, bucketName)

	if w1.Code != http.StatusOK {
		t.Fatalf("第一次创建应该成功: got %d", w1.Code)
	}

	// 第二次创建应该返回冲突
	req2 := httptest.NewRequest("PUT", "/"+bucketName, nil)
	w2 := httptest.NewRecorder()
	server.handleCreateBucket(w2, req2, bucketName)

	if w2.Code != http.StatusConflict {
		t.Errorf("重复创建应该返回409冲突: got %d", w2.Code)
	}
}

// TestHandleDeleteBucket 测试删除存储桶
func TestHandleDeleteBucket(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	t.Run("删除存在的空桶", func(t *testing.T) {
		bucketName := "bucket-to-delete"
		createTestBucket(t, server, bucketName)

		req := httptest.NewRequest("DELETE", "/"+bucketName, nil)
		w := httptest.NewRecorder()

		server.handleDeleteBucket(w, req, bucketName)

		if w.Code != http.StatusNoContent {
			t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusNoContent)
		}

		// 验证桶已删除
		headReq := httptest.NewRequest("HEAD", "/"+bucketName, nil)
		headW := httptest.NewRecorder()
		server.handleHeadBucket(headW, headReq, bucketName)

		if headW.Code != http.StatusNotFound {
			t.Errorf("桶应该已被删除: got %d", headW.Code)
		}
	})

	t.Run("删除不存在的桶", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/non-existent", nil)
		w := httptest.NewRecorder()

		server.handleDeleteBucket(w, req, "non-existent")

		if w.Code != http.StatusNotFound {
			t.Errorf("应该返回404: got %d", w.Code)
		}
	})
}

// TestHandleHeadBucket 测试检查存储桶存在
func TestHandleHeadBucket(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	t.Run("桶存在", func(t *testing.T) {
		bucketName := "existing-bucket"
		createTestBucket(t, server, bucketName)

		req := httptest.NewRequest("HEAD", "/"+bucketName, nil)
		w := httptest.NewRecorder()

		server.handleHeadBucket(w, req, bucketName)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusOK)
		}

		// 验证区域头
		region := w.Header().Get("x-amz-bucket-region")
		if region != config.Global.Server.Region {
			t.Errorf("区域头不正确: got %s, want %s", region, config.Global.Server.Region)
		}
	})

	t.Run("桶不存在", func(t *testing.T) {
		req := httptest.NewRequest("HEAD", "/non-existent", nil)
		w := httptest.NewRecorder()

		server.handleHeadBucket(w, req, "non-existent")

		if w.Code != http.StatusNotFound {
			t.Errorf("应该返回404: got %d", w.Code)
		}
	})
}

// TestHandleListObjects 测试列举对象
func TestHandleListObjects(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	bucketName := "list-test-bucket"
	createTestBucket(t, server, bucketName)

	t.Run("空桶列表V1", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/"+bucketName, nil)
		w := httptest.NewRecorder()

		server.handleListObjects(w, req, bucketName)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d", w.Code)
		}

		var result ListBucketResult
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if result.Name != bucketName {
			t.Errorf("桶名不匹配: got %s", result.Name)
		}

		if len(result.Contents) != 0 {
			t.Errorf("应该没有对象: got %d", len(result.Contents))
		}
	})

	t.Run("空桶列表V2", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/"+bucketName+"?list-type=2", nil)
		w := httptest.NewRecorder()

		server.handleListObjects(w, req, bucketName)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d", w.Code)
		}

		var result ListBucketResultV2
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if result.Name != bucketName {
			t.Errorf("桶名不匹配: got %s", result.Name)
		}

		if result.KeyCount != 0 {
			t.Errorf("KeyCount应该是0: got %d", result.KeyCount)
		}
	})

	t.Run("不存在的桶", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/non-existent", nil)
		w := httptest.NewRecorder()

		server.handleListObjects(w, req, "non-existent")

		if w.Code != http.StatusNotFound {
			t.Errorf("应该返回404: got %d", w.Code)
		}
	})
}

// TestHandleListObjectsWithParams 测试带参数的列举对象
func TestHandleListObjectsWithParams(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	bucketName := "params-test-bucket"
	createTestBucket(t, server, bucketName)

	t.Run("带prefix参数", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/"+bucketName+"?prefix=folder/", nil)
		w := httptest.NewRecorder()

		server.handleListObjects(w, req, bucketName)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d", w.Code)
		}

		var result ListBucketResult
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if result.Prefix != "folder/" {
			t.Errorf("Prefix不匹配: got %s", result.Prefix)
		}
	})

	t.Run("带max-keys参数", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/"+bucketName+"?max-keys=10", nil)
		w := httptest.NewRecorder()

		server.handleListObjects(w, req, bucketName)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d", w.Code)
		}

		var result ListBucketResult
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if result.MaxKeys != 10 {
			t.Errorf("MaxKeys不匹配: got %d, want 10", result.MaxKeys)
		}
	})

	t.Run("带delimiter参数", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/"+bucketName+"?delimiter=/", nil)
		w := httptest.NewRecorder()

		server.handleListObjects(w, req, bucketName)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d", w.Code)
		}
	})

	t.Run("V2带continuation-token参数", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/"+bucketName+"?list-type=2&continuation-token=abc", nil)
		w := httptest.NewRecorder()

		server.handleListObjects(w, req, bucketName)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不正确: got %d", w.Code)
		}

		var result ListBucketResultV2
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if result.ContinuationToken != "abc" {
			t.Errorf("ContinuationToken不匹配: got %s", result.ContinuationToken)
		}
	})
}

// TestListBucketResultXML 测试XML序列化
func TestListBucketResultXML(t *testing.T) {
	result := ListBucketResult{
		Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:        "test-bucket",
		Prefix:      "folder/",
		Marker:      "",
		MaxKeys:     1000,
		IsTruncated: false,
		Contents: []ObjectInfo{
			{
				Key:          "folder/file.txt",
				LastModified: time.Now().UTC().Format(time.RFC3339),
				ETag:         `"d41d8cd98f00b204e9800998ecf8427e"`,
				Size:         1024,
				StorageClass: "STANDARD",
			},
		},
	}

	data, err := xml.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	if !strings.Contains(string(data), "ListBucketResult") {
		t.Error("XML应该包含ListBucketResult")
	}

	if !strings.Contains(string(data), "test-bucket") {
		t.Error("XML应该包含桶名")
	}

	if !strings.Contains(string(data), "folder/file.txt") {
		t.Error("XML应该包含对象key")
	}
}

// TestListAllMyBucketsResultXML 测试XML序列化
func TestListAllMyBucketsResultXML(t *testing.T) {
	result := ListAllMyBucketsResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Owner: Owner{
			ID:          "test-owner",
			DisplayName: "Test Owner",
		},
		Buckets: Buckets{
			Bucket: []BucketInfo{
				{
					Name:         "bucket1",
					CreationDate: time.Now().UTC().Format(time.RFC3339),
				},
				{
					Name:         "bucket2",
					CreationDate: time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	}

	data, err := xml.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	if !strings.Contains(string(data), "ListAllMyBucketsResult") {
		t.Error("XML应该包含ListAllMyBucketsResult")
	}

	if !strings.Contains(string(data), "test-owner") {
		t.Error("XML应该包含所有者ID")
	}

	if !strings.Contains(string(data), "bucket1") {
		t.Error("XML应该包含桶名")
	}
}

// TestConcurrentBucketCreation 测试并发创建桶
func TestConcurrentBucketCreation(t *testing.T) {
	server, cleanup := setupBucketTestServer(t)
	defer cleanup()

	const numGoroutines = 10
	results := make(chan int, numGoroutines)

	// 并发创建同名桶
	for i := 0; i < numGoroutines; i++ {
		go func() {
			req := httptest.NewRequest("PUT", "/concurrent-bucket", nil)
			w := httptest.NewRecorder()
			server.handleCreateBucket(w, req, "concurrent-bucket")
			results <- w.Code
		}()
	}

	// 收集结果
	successCount := 0
	conflictCount := 0
	for i := 0; i < numGoroutines; i++ {
		code := <-results
		switch code {
		case http.StatusOK:
			successCount++
		case http.StatusConflict:
			conflictCount++
		}
	}

	// 只应该有一个成功
	if successCount != 1 {
		t.Errorf("应该只有1个成功创建: got %d", successCount)
	}

	// 其他应该是冲突
	if conflictCount != numGoroutines-1 {
		t.Errorf("应该有%d个冲突: got %d", numGoroutines-1, conflictCount)
	}
}

// BenchmarkHandleListBuckets 列举桶性能测试
func BenchmarkHandleListBuckets(b *testing.B) {
	// 初始化配置
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/test.db")
	filestore, _ := storage.NewFileStore(tempDir)
	defer metadata.Close()

	server := NewServer(metadata, filestore)

	// 创建一些测试桶
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("PUT", "/bucket"+string(rune('a'+i%26))+string(rune('0'+i/26)), nil)
		w := httptest.NewRecorder()
		server.handleCreateBucket(w, req, "bucket"+string(rune('a'+i%26))+string(rune('0'+i/26)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		server.handleListBuckets(w, req)
	}
}

// BenchmarkHandleCreateBucket 创建桶性能测试
func BenchmarkHandleCreateBucket(b *testing.B) {
	// 初始化配置
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/test.db")
	filestore, _ := storage.NewFileStore(tempDir)
	defer metadata.Close()

	server := NewServer(metadata, filestore)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucketName := "bucket-" + strings.Repeat("a", i%10+1) + "-" + strings.Repeat("b", (i/10)%10+1)
		req := httptest.NewRequest("PUT", "/"+bucketName, nil)
		w := httptest.NewRecorder()
		server.handleCreateBucket(w, req, bucketName)
	}
}

// BenchmarkHandleHeadBucket HEAD桶性能测试
func BenchmarkHandleHeadBucket(b *testing.B) {
	// 初始化配置
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/test.db")
	filestore, _ := storage.NewFileStore(tempDir)
	defer metadata.Close()

	server := NewServer(metadata, filestore)

	// 创建测试桶
	req := httptest.NewRequest("PUT", "/test-bucket", nil)
	w := httptest.NewRecorder()
	server.handleCreateBucket(w, req, "test-bucket")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("HEAD", "/test-bucket", nil)
		w := httptest.NewRecorder()
		server.handleHeadBucket(w, req, "test-bucket")
	}
}
