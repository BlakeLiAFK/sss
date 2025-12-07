package api

import (
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// setupObjectTestServer 初始化对象测试服务器
func setupObjectTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	// 确保配置已初始化
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := t.TempDir()
	metadata, err := storage.NewMetadataStore(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("创建MetadataStore失败: %v", err)
	}

	filestore, err := storage.NewFileStore(tempDir)
	if err != nil {
		metadata.Close()
		t.Fatalf("创建FileStore失败: %v", err)
	}

	server := NewServer(metadata, filestore)
	cleanup := func() {
		metadata.Close()
	}

	return server, cleanup
}

// createTestBucketAndObject 创建测试桶和对象
func createTestBucketAndObject(t *testing.T, s *Server, bucket, key string, content []byte) {
	t.Helper()

	// 创建桶
	if err := s.metadata.CreateBucket(bucket); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 上传对象
	storagePath, etag, err := s.filestore.PutObject(bucket, key, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("上传对象失败: %v", err)
	}

	obj := &storage.Object{
		Key:         key,
		Bucket:      bucket,
		Size:        int64(len(content)),
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	if err := s.metadata.PutObject(obj); err != nil {
		t.Fatalf("保存对象元数据失败: %v", err)
	}
}

// TestHandleGetObject 测试获取对象
func TestHandleGetObject(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	// 准备测试数据
	testContent := []byte("Hello, World! This is a test object content.")
	createTestBucketAndObject(t, server, "test-bucket", "test-key.txt", testContent)

	tests := []struct {
		name           string
		bucket         string
		key            string
		rangeHeader    string
		expectedStatus int
		expectedBody   string
		checkHeaders   map[string]string
	}{
		{
			name:           "获取完整对象",
			bucket:         "test-bucket",
			key:            "test-key.txt",
			rangeHeader:    "",
			expectedStatus: http.StatusOK,
			expectedBody:   string(testContent),
			checkHeaders: map[string]string{
				"Content-Type":   "text/plain",
				"Accept-Ranges":  "bytes",
			},
		},
		{
			name:           "Range请求-前10字节",
			bucket:         "test-bucket",
			key:            "test-key.txt",
			rangeHeader:    "bytes=0-9",
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "Hello, Wor",
			checkHeaders: map[string]string{
				"Content-Range": "bytes 0-9/" + strconv.Itoa(len(testContent)),
			},
		},
		{
			name:           "Range请求-中间部分",
			bucket:         "test-bucket",
			key:            "test-key.txt",
			rangeHeader:    "bytes=7-11",
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "World",
		},
		{
			name:           "Range请求-从某位置到结尾",
			bucket:         "test-bucket",
			key:            "test-key.txt",
			rangeHeader:    "bytes=37-",
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "ontent.",
		},
		{
			name:           "不存在的桶",
			bucket:         "nonexistent-bucket",
			key:            "test-key.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "不存在的对象",
			bucket:         "test-bucket",
			key:            "nonexistent-key.txt",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tc.bucket+"/"+tc.key, nil)
			if tc.rangeHeader != "" {
				req.Header.Set("Range", tc.rangeHeader)
			}
			rec := httptest.NewRecorder()

			server.handleGetObject(rec, req, tc.bucket, tc.key)

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectedBody != "" && rec.Body.String() != tc.expectedBody {
				t.Errorf("响应体错误: 期望 %q, 实际 %q", tc.expectedBody, rec.Body.String())
			}

			for header, expected := range tc.checkHeaders {
				if got := rec.Header().Get(header); got != expected {
					t.Errorf("Header %s 错误: 期望 %q, 实际 %q", header, expected, got)
				}
			}
		})
	}
}

// TestHandleGetObjectRangeEdgeCases 测试Range请求边界情况
func TestHandleGetObjectRangeEdgeCases(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	content := []byte("0123456789")
	createTestBucketAndObject(t, server, "range-test", "data.bin", content)

	tests := []struct {
		name           string
		rangeHeader    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "超出范围的end",
			rangeHeader:    "bytes=0-100",
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "0123456789",
		},
		{
			name:           "无效范围-start大于end",
			rangeHeader:    "bytes=8-5",
			expectedStatus: http.StatusRequestedRangeNotSatisfiable,
		},
		{
			name:           "只有start",
			rangeHeader:    "bytes=5-",
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "56789",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/range-test/data.bin", nil)
			req.Header.Set("Range", tc.rangeHeader)
			rec := httptest.NewRecorder()

			server.handleGetObject(rec, req, "range-test", "data.bin")

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectedBody != "" && rec.Body.String() != tc.expectedBody {
				t.Errorf("响应体错误: 期望 %q, 实际 %q", tc.expectedBody, rec.Body.String())
			}
		})
	}
}

// TestHandlePutObject 测试上传对象
func TestHandlePutObject(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("upload-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	tests := []struct {
		name           string
		bucket         string
		key            string
		content        []byte
		contentType    string
		expectedStatus int
	}{
		{
			name:           "上传普通文本文件",
			bucket:         "upload-bucket",
			key:            "hello.txt",
			content:        []byte("Hello, World!"),
			contentType:    "text/plain",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "上传二进制文件",
			bucket:         "upload-bucket",
			key:            "binary.bin",
			content:        []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE},
			contentType:    "application/octet-stream",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "上传空文件",
			bucket:         "upload-bucket",
			key:            "empty.txt",
			content:        []byte{},
			contentType:    "text/plain",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "上传到不存在的桶",
			bucket:         "nonexistent-bucket",
			key:            "file.txt",
			content:        []byte("content"),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "上传带中文路径的文件",
			bucket:         "upload-bucket",
			key:            "文档/测试文件.txt",
			content:        []byte("中文内容"),
			contentType:    "text/plain; charset=utf-8",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/"+tc.bucket+"/"+tc.key, bytes.NewReader(tc.content))
			req.ContentLength = int64(len(tc.content))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			server.handlePutObject(rec, req, tc.bucket, tc.key)

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d, 响应: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			// 验证成功上传后的ETag
			if tc.expectedStatus == http.StatusOK {
				etag := rec.Header().Get("ETag")
				if etag == "" {
					t.Error("成功上传后应返回ETag")
				}

				// 验证对象已创建
				obj, err := server.metadata.GetObject(tc.bucket, tc.key)
				if err != nil {
					t.Errorf("获取对象元数据失败: %v", err)
				}
				if obj == nil {
					t.Error("对象未创建")
				} else if obj.Size != int64(len(tc.content)) {
					t.Errorf("对象大小错误: 期望 %d, 实际 %d", len(tc.content), obj.Size)
				}
			}
		})
	}
}

// TestHandlePutObjectWithSizeLimit 测试上传对象大小限制
func TestHandlePutObjectWithSizeLimit(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("limit-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 保存原始配置
	origMaxUpload := config.Global.Storage.MaxUploadSize
	origMaxObject := config.Global.Storage.MaxObjectSize
	defer func() {
		config.Global.Storage.MaxUploadSize = origMaxUpload
		config.Global.Storage.MaxObjectSize = origMaxObject
	}()

	t.Run("超过MaxUploadSize限制", func(t *testing.T) {
		config.Global.Storage.MaxUploadSize = 100
		config.Global.Storage.MaxObjectSize = 0

		content := make([]byte, 200)
		req := httptest.NewRequest(http.MethodPut, "/limit-bucket/big.bin", bytes.NewReader(content))
		req.ContentLength = int64(len(content))
		rec := httptest.NewRecorder()

		server.handlePutObject(rec, req, "limit-bucket", "big.bin")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("预签名URL大小限制", func(t *testing.T) {
		config.Global.Storage.MaxUploadSize = 0
		config.Global.Storage.MaxObjectSize = 0

		content := make([]byte, 200)
		req := httptest.NewRequest(http.MethodPut, "/limit-bucket/presigned.bin?X-Amz-Max-Content-Length=100", bytes.NewReader(content))
		req.ContentLength = int64(len(content))
		rec := httptest.NewRecorder()

		server.handlePutObject(rec, req, "limit-bucket", "presigned.bin")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("预签名URL内容类型限制", func(t *testing.T) {
		content := []byte("test")
		req := httptest.NewRequest(http.MethodPut, "/limit-bucket/typed.bin?X-Amz-Content-Type=application/json", bytes.NewReader(content))
		req.ContentLength = int64(len(content))
		req.Header.Set("Content-Type", "text/plain") // 不匹配
		rec := httptest.NewRecorder()

		server.handlePutObject(rec, req, "limit-bucket", "typed.bin")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestHandleDeleteObject 测试删除对象
func TestHandleDeleteObject(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	// 创建测试桶和对象
	createTestBucketAndObject(t, server, "delete-bucket", "to-delete.txt", []byte("delete me"))

	tests := []struct {
		name           string
		bucket         string
		key            string
		expectedStatus int
	}{
		{
			name:           "删除存在的对象",
			bucket:         "delete-bucket",
			key:            "to-delete.txt",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "删除不存在的对象-S3语义返回204",
			bucket:         "delete-bucket",
			key:            "nonexistent.txt",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/"+tc.bucket+"/"+tc.key, nil)
			rec := httptest.NewRecorder()

			server.handleDeleteObject(rec, req, tc.bucket, tc.key)

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d", tc.expectedStatus, rec.Code)
			}
		})
	}

	// 验证对象已删除
	t.Run("验证对象已删除", func(t *testing.T) {
		obj, err := server.metadata.GetObject("delete-bucket", "to-delete.txt")
		if err != nil {
			t.Errorf("获取对象元数据失败: %v", err)
		}
		if obj != nil {
			t.Error("对象应已被删除")
		}
	})
}

// TestHandleCopyObject 测试复制对象
func TestHandleCopyObject(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	// 创建源桶和对象
	createTestBucketAndObject(t, server, "src-bucket", "original.txt", []byte("Original content"))

	// 创建目标桶
	if err := server.metadata.CreateBucket("dest-bucket"); err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	tests := []struct {
		name           string
		destBucket     string
		destKey        string
		copySource     string
		expectedStatus int
	}{
		{
			name:           "跨桶复制",
			destBucket:     "dest-bucket",
			destKey:        "copied.txt",
			copySource:     "/src-bucket/original.txt",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "同桶复制-无斜杠前缀",
			destBucket:     "src-bucket",
			destKey:        "copy-in-same.txt",
			copySource:     "src-bucket/original.txt",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "复制到不存在的桶",
			destBucket:     "nonexistent-bucket",
			destKey:        "copied.txt",
			copySource:     "/src-bucket/original.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "从不存在的桶复制",
			destBucket:     "dest-bucket",
			destKey:        "copied.txt",
			copySource:     "/nonexistent-bucket/original.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "复制不存在的对象",
			destBucket:     "dest-bucket",
			destKey:        "copied.txt",
			copySource:     "/src-bucket/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "缺少copy-source头",
			destBucket:     "dest-bucket",
			destKey:        "copied.txt",
			copySource:     "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "无效的copy-source格式",
			destBucket:     "dest-bucket",
			destKey:        "copied.txt",
			copySource:     "invalid-format",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/"+tc.destBucket+"/"+tc.destKey, nil)
			if tc.copySource != "" {
				req.Header.Set("x-amz-copy-source", tc.copySource)
			}
			rec := httptest.NewRecorder()

			server.handleCopyObject(rec, req, tc.destBucket, tc.destKey)

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d, 响应: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			// 验证成功复制后的响应
			if tc.expectedStatus == http.StatusOK {
				body := rec.Body.String()
				if !strings.Contains(body, "<CopyObjectResult>") {
					t.Error("响应应包含CopyObjectResult")
				}
				if !strings.Contains(body, "<ETag>") {
					t.Error("响应应包含ETag")
				}
			}
		})
	}
}

// TestHandleCopyObjectPathTraversal 测试复制对象路径遍历防护
func TestHandleCopyObjectPathTraversal(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("secure-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	tests := []struct {
		name       string
		copySource string
	}{
		{
			name:       "桶名包含路径遍历",
			copySource: "/../../etc/bucket/file.txt",
		},
		{
			name:       "key包含路径遍历",
			copySource: "/secure-bucket/../../../etc/passwd",
		},
		{
			name:       "桶名包含反斜杠",
			copySource: "/bucket\\path/file.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/secure-bucket/target.txt", nil)
			req.Header.Set("x-amz-copy-source", tc.copySource)
			rec := httptest.NewRecorder()

			server.handleCopyObject(rec, req, "secure-bucket", "target.txt")

			if rec.Code != http.StatusBadRequest {
				t.Errorf("应拒绝路径遍历攻击: 状态码 %d, 响应: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestHandleCopyObjectURLEncoding 测试复制对象URL编码
func TestHandleCopyObjectURLEncoding(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	// 创建桶和带中文名的对象
	createTestBucketAndObject(t, server, "encoding-bucket", "中文文件.txt", []byte("中文内容"))

	t.Run("URL编码的中文路径", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/encoding-bucket/copied-file.txt", nil)
		// URL编码的中文
		req.Header.Set("x-amz-copy-source", "/encoding-bucket/%E4%B8%AD%E6%96%87%E6%96%87%E4%BB%B6.txt")
		rec := httptest.NewRecorder()

		server.handleCopyObject(rec, req, "encoding-bucket", "copied-file.txt")

		if rec.Code != http.StatusOK {
			t.Errorf("应支持URL编码路径: 状态码 %d, 响应: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestHandleHeadObject 测试获取对象元数据
func TestHandleHeadObject(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	testContent := []byte("Test content for HEAD request")
	createTestBucketAndObject(t, server, "head-bucket", "head-test.txt", testContent)

	tests := []struct {
		name           string
		bucket         string
		key            string
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name:           "获取存在对象的元数据",
			bucket:         "head-bucket",
			key:            "head-test.txt",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name:           "不存在的桶",
			bucket:         "nonexistent-bucket",
			key:            "head-test.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "不存在的对象",
			bucket:         "head-bucket",
			key:            "nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodHead, "/"+tc.bucket+"/"+tc.key, nil)
			rec := httptest.NewRecorder()

			server.handleHeadObject(rec, req, tc.bucket, tc.key)

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d", tc.expectedStatus, rec.Code)
			}

			if tc.checkHeaders {
				// HEAD请求不应有响应体
				if rec.Body.Len() != 0 {
					t.Error("HEAD请求不应返回响应体")
				}

				// 验证必要的头
				if rec.Header().Get("Content-Length") != strconv.Itoa(len(testContent)) {
					t.Errorf("Content-Length错误: %s", rec.Header().Get("Content-Length"))
				}
				if rec.Header().Get("Content-Type") != "text/plain" {
					t.Errorf("Content-Type错误: %s", rec.Header().Get("Content-Type"))
				}
				if rec.Header().Get("ETag") == "" {
					t.Error("应返回ETag头")
				}
				if rec.Header().Get("Accept-Ranges") != "bytes" {
					t.Error("应返回Accept-Ranges: bytes")
				}
			}
		})
	}
}

// TestObjectOverwrite 测试覆盖已存在的对象
func TestObjectOverwrite(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	if err := server.metadata.CreateBucket("overwrite-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 第一次上传
	content1 := []byte("Original content")
	req1 := httptest.NewRequest(http.MethodPut, "/overwrite-bucket/file.txt", bytes.NewReader(content1))
	req1.ContentLength = int64(len(content1))
	rec1 := httptest.NewRecorder()
	server.handlePutObject(rec1, req1, "overwrite-bucket", "file.txt")

	if rec1.Code != http.StatusOK {
		t.Fatalf("第一次上传失败: %d", rec1.Code)
	}
	etag1 := rec1.Header().Get("ETag")

	// 第二次上传（覆盖）
	content2 := []byte("Updated content with different size")
	req2 := httptest.NewRequest(http.MethodPut, "/overwrite-bucket/file.txt", bytes.NewReader(content2))
	req2.ContentLength = int64(len(content2))
	rec2 := httptest.NewRecorder()
	server.handlePutObject(rec2, req2, "overwrite-bucket", "file.txt")

	if rec2.Code != http.StatusOK {
		t.Fatalf("覆盖上传失败: %d", rec2.Code)
	}
	etag2 := rec2.Header().Get("ETag")

	// 验证ETag已更新
	if etag1 == etag2 {
		t.Error("覆盖后ETag应该改变")
	}

	// 验证可以获取新内容
	reqGet := httptest.NewRequest(http.MethodGet, "/overwrite-bucket/file.txt", nil)
	recGet := httptest.NewRecorder()
	server.handleGetObject(recGet, reqGet, "overwrite-bucket", "file.txt")

	if recGet.Body.String() != string(content2) {
		t.Errorf("获取的内容不是最新的: 期望 %q, 实际 %q", string(content2), recGet.Body.String())
	}
}

// TestLargeObjectOperations 测试大对象操作
func TestLargeObjectOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大对象测试")
	}

	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	if err := server.metadata.CreateBucket("large-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 1MB 测试数据
	largeContent := make([]byte, 1024*1024)
	if _, err := rand.Read(largeContent); err != nil {
		t.Fatalf("生成随机数据失败: %v", err)
	}

	t.Run("上传大文件", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/large-bucket/large.bin", bytes.NewReader(largeContent))
		req.ContentLength = int64(len(largeContent))
		rec := httptest.NewRecorder()

		server.handlePutObject(rec, req, "large-bucket", "large.bin")

		if rec.Code != http.StatusOK {
			t.Errorf("上传大文件失败: %d", rec.Code)
		}
	})

	t.Run("下载大文件", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/large-bucket/large.bin", nil)
		rec := httptest.NewRecorder()

		server.handleGetObject(rec, req, "large-bucket", "large.bin")

		if rec.Code != http.StatusOK {
			t.Errorf("下载大文件失败: %d", rec.Code)
		}
		if rec.Body.Len() != len(largeContent) {
			t.Errorf("下载大小错误: 期望 %d, 实际 %d", len(largeContent), rec.Body.Len())
		}
	})

	t.Run("Range请求大文件", func(t *testing.T) {
		// 请求中间部分
		start := int64(512 * 1024)
		end := int64(512*1024 + 1024 - 1)

		req := httptest.NewRequest(http.MethodGet, "/large-bucket/large.bin", nil)
		req.Header.Set("Range", "bytes="+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10))
		rec := httptest.NewRecorder()

		server.handleGetObject(rec, req, "large-bucket", "large.bin")

		if rec.Code != http.StatusPartialContent {
			t.Errorf("Range请求状态码错误: 期望 %d, 实际 %d", http.StatusPartialContent, rec.Code)
		}

		expectedLen := end - start + 1
		if rec.Body.Len() != int(expectedLen) {
			t.Errorf("Range响应大小错误: 期望 %d, 实际 %d", expectedLen, rec.Body.Len())
		}

		// 验证内容正确
		if !bytes.Equal(rec.Body.Bytes(), largeContent[start:end+1]) {
			t.Error("Range请求返回的内容与原始数据不匹配")
		}
	})
}

// TestConcurrentObjectOperations 测试并发对象操作
func TestConcurrentObjectOperations(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	if err := server.metadata.CreateBucket("concurrent-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	numGoroutines := 10
	numOperations := 5
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := "concurrent-" + strconv.Itoa(id) + "-" + strconv.Itoa(j) + ".txt"
				content := []byte("Content from goroutine " + strconv.Itoa(id))

				// 上传
				reqPut := httptest.NewRequest(http.MethodPut, "/concurrent-bucket/"+key, bytes.NewReader(content))
				reqPut.ContentLength = int64(len(content))
				recPut := httptest.NewRecorder()
				server.handlePutObject(recPut, reqPut, "concurrent-bucket", key)

				if recPut.Code != http.StatusOK {
					t.Errorf("并发上传失败: goroutine=%d, op=%d, status=%d", id, j, recPut.Code)
					continue
				}

				// 下载验证
				reqGet := httptest.NewRequest(http.MethodGet, "/concurrent-bucket/"+key, nil)
				recGet := httptest.NewRecorder()
				server.handleGetObject(recGet, reqGet, "concurrent-bucket", key)

				if recGet.Code != http.StatusOK {
					t.Errorf("并发下载失败: goroutine=%d, op=%d, status=%d", id, j, recGet.Code)
				}
			}
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestSpecialCharacterKeys 测试特殊字符对象键
func TestSpecialCharacterKeys(t *testing.T) {
	server, cleanup := setupObjectTestServer(t)
	defer cleanup()

	if err := server.metadata.CreateBucket("special-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	keys := []string{
		"path/to/file.txt",
		"file with spaces.txt",
		"文件名.txt",
		"file-with-dashes_and_underscores.txt",
		"deep/nested/path/file.txt",
	}

	for _, key := range keys {
		t.Run("key:"+key, func(t *testing.T) {
			content := []byte("Content for " + key)

			// URL 编码 key，防止空格等特殊字符导致请求构造失败
			encodedKey := url.PathEscape(key)

			// 上传
			reqPut := httptest.NewRequest(http.MethodPut, "/special-bucket/"+encodedKey, bytes.NewReader(content))
			reqPut.ContentLength = int64(len(content))
			recPut := httptest.NewRecorder()
			server.handlePutObject(recPut, reqPut, "special-bucket", key)

			if recPut.Code != http.StatusOK {
				t.Errorf("上传失败: %d", recPut.Code)
				return
			}

			// 下载验证
			reqGet := httptest.NewRequest(http.MethodGet, "/special-bucket/"+encodedKey, nil)
			recGet := httptest.NewRecorder()
			server.handleGetObject(recGet, reqGet, "special-bucket", key)

			if recGet.Code != http.StatusOK {
				t.Errorf("下载失败: %d", recGet.Code)
			}
			if recGet.Body.String() != string(content) {
				t.Errorf("内容不匹配: 期望 %q, 实际 %q", string(content), recGet.Body.String())
			}
		})
	}
}

// BenchmarkHandleGetObject 基准测试-获取对象
func BenchmarkHandleGetObject(b *testing.B) {
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/bench.db")
	defer metadata.Close()
	filestore, _ := storage.NewFileStore(tempDir)
	server := NewServer(metadata, filestore)

	// 创建测试数据
	metadata.CreateBucket("bench-bucket")
	content := bytes.Repeat([]byte("x"), 4096) // 4KB
	storagePath, etag, _ := filestore.PutObject("bench-bucket", "bench.bin", bytes.NewReader(content), 4096)
	obj := &storage.Object{
		Key:         "bench.bin",
		Bucket:      "bench-bucket",
		Size:        4096,
		ETag:        etag,
		ContentType: "application/octet-stream",
		StoragePath: storagePath,
	}
	metadata.PutObject(obj)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/bench-bucket/bench.bin", nil)
		rec := httptest.NewRecorder()
		server.handleGetObject(rec, req, "bench-bucket", "bench.bin")
		// 消费响应体
		io.Copy(io.Discard, rec.Body)
	}
}

// BenchmarkHandlePutObject 基准测试-上传对象
func BenchmarkHandlePutObject(b *testing.B) {
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/bench.db")
	defer metadata.Close()
	filestore, _ := storage.NewFileStore(tempDir)
	server := NewServer(metadata, filestore)

	metadata.CreateBucket("bench-bucket")
	content := bytes.Repeat([]byte("x"), 4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench-" + strconv.Itoa(i) + ".bin"
		req := httptest.NewRequest(http.MethodPut, "/bench-bucket/"+key, bytes.NewReader(content))
		req.ContentLength = 4096
		rec := httptest.NewRecorder()
		server.handlePutObject(rec, req, "bench-bucket", key)
	}
}

// BenchmarkHandleHeadObject 基准测试-HEAD请求
func BenchmarkHandleHeadObject(b *testing.B) {
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/bench.db")
	defer metadata.Close()
	filestore, _ := storage.NewFileStore(tempDir)
	server := NewServer(metadata, filestore)

	metadata.CreateBucket("bench-bucket")
	content := []byte("test")
	storagePath, etag, _ := filestore.PutObject("bench-bucket", "bench.txt", bytes.NewReader(content), 4)
	obj := &storage.Object{
		Key:         "bench.txt",
		Bucket:      "bench-bucket",
		Size:        4,
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	metadata.PutObject(obj)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodHead, "/bench-bucket/bench.txt", nil)
		rec := httptest.NewRecorder()
		server.handleHeadObject(rec, req, "bench-bucket", "bench.txt")
	}
}
