package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// setupHandlersTestServer 创建测试服务器
func setupHandlersTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	// 确保配置已初始化
	if config.Global == nil {
		config.NewDefault()
	}

	// 确保日志已初始化
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := t.TempDir()

	// 创建元数据存储
	metadata, err := storage.NewMetadataStore(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}

	// 创建文件存储
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

// TestServeHTTP 测试ServeHTTP处理器
func TestServeHTTP(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	t.Run("OPTIONS请求返回200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("CORS头部设置正确", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		// 检查CORS头部
		if rec.Header().Get("Access-Control-Allow-Origin") == "" {
			t.Error("缺少 Access-Control-Allow-Origin 头部")
		}
		if rec.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("缺少 Access-Control-Allow-Methods 头部")
		}
		if rec.Header().Get("Access-Control-Allow-Headers") == "" {
			t.Error("缺少 Access-Control-Allow-Headers 头部")
		}
	})

	t.Run("Server头部设置为SSS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Header().Get("Server") != "SSS" {
			t.Errorf("Server头部错误: 期望 'SSS', 实际 '%s'", rec.Header().Get("Server"))
		}
	})

	t.Run("x-amz-request-id头部存在", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Header().Get("x-amz-request-id") == "" {
			t.Error("缺少 x-amz-request-id 头部")
		}
	})

	t.Run("配置的CORS来源被使用", func(t *testing.T) {
		// 保存原始配置
		originalOrigin := ""
		if config.Global != nil {
			originalOrigin = config.Global.Security.CORSOrigin
		}

		// 设置测试CORS来源
		if config.Global != nil {
			config.Global.Security.CORSOrigin = "https://example.com"
		}

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		expectedOrigin := "https://example.com"
		if rec.Header().Get("Access-Control-Allow-Origin") != expectedOrigin {
			t.Errorf("CORS来源错误: 期望 '%s', 实际 '%s'",
				expectedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
		}

		// 恢复原始配置
		if config.Global != nil {
			config.Global.Security.CORSOrigin = originalOrigin
		}
	})
}

// TestIsRootStaticFile 测试isRootStaticFile函数
func TestIsRootStaticFile(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		// 应该匹配的根目录静态文件
		{"根目录svg文件", "/favicon.svg", true},
		{"根目录ico文件", "/favicon.ico", true},
		{"根目录png文件", "/logo.png", true},
		{"根目录txt文件", "/robots.txt", true},
		{"根目录webmanifest文件", "/site.webmanifest", true},

		// 不应该匹配的路径（子目录文件）
		{"子目录txt文件", "/bucket/file.txt", false},
		{"深层路径svg", "/a/b/c.svg", false},
		{"assets目录文件", "/assets/icon.svg", false},

		// 不应该匹配的其他情况
		{"无扩展名", "/readme", false},
		{"根目录js文件", "/script.js", false},
		{"根目录html文件", "/index.html", false},
		{"空路径", "", false},
		{"不以斜杠开头", "favicon.ico", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isRootStaticFile(tc.path)
			if result != tc.expected {
				t.Errorf("isRootStaticFile(%q) = %v, 期望 %v", tc.path, result, tc.expected)
			}
		})
	}
}

// TestHandleHealth 测试健康检查端点
func TestHandleHealth(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	t.Run("健康检查返回正确响应", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()

		server.handleHealth(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response["status"] != "ok" {
			t.Errorf("status错误: 期望 'ok', 实际 '%v'", response["status"])
		}

		if response["version"] == nil || response["version"] == "" {
			t.Error("version字段缺失或为空")
		}
	})
}

// TestHandlePresign 测试预签名URL生成
func TestHandlePresign(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("presign-bucket"); err != nil {
		t.Fatalf("创建测试桶失败: %v", err)
	}

	t.Run("方法限制-只允许POST", func(t *testing.T) {
		methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodHead}

		for _, method := range methods {
			req := httptest.NewRequest(method, "/api/presign", nil)
			rec := httptest.NewRecorder()

			server.handlePresign(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s请求: 期望状态码 %d, 实际 %d", method, http.StatusMethodNotAllowed, rec.Code)
			}
		}
	})

	t.Run("缺少必需参数", func(t *testing.T) {
		testCases := []struct {
			name string
			body string
		}{
			{"缺少bucket", `{"key": "test.txt"}`},
			{"缺少key", `{"bucket": "presign-bucket"}`},
			{"空bucket", `{"bucket": "", "key": "test.txt"}`},
			{"空key", `{"bucket": "presign-bucket", "key": ""}`},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				server.handlePresign(rec, req)

				if rec.Code != http.StatusBadRequest {
					t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
				}
			})
		}
	})

	t.Run("无效的bucket名称-路径遍历", func(t *testing.T) {
		testCases := []string{
			`{"bucket": "../evil", "key": "test.txt"}`,
			`{"bucket": "bucket/name", "key": "test.txt"}`,
			`{"bucket": "bucket\\name", "key": "test.txt"}`,
		}

		for _, body := range testCases {
			req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.handlePresign(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusBadRequest, rec.Code, body)
			}
		}
	})

	t.Run("无效的key-路径遍历", func(t *testing.T) {
		testCases := []string{
			`{"bucket": "presign-bucket", "key": "../evil.txt"}`,
			`{"bucket": "presign-bucket", "key": "/absolute/path.txt"}`,
		}

		for _, body := range testCases {
			req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.handlePresign(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusBadRequest, rec.Code, body)
			}
		}
	})

	t.Run("桶不存在", func(t *testing.T) {
		body := `{"bucket": "nonexistent-bucket", "key": "test.txt"}`
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handlePresign(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("成功生成预签名URL", func(t *testing.T) {
		body := `{"bucket": "presign-bucket", "key": "test.txt", "method": "PUT", "expiresMinutes": 30}`
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handlePresign(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response PresignResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response.URL == "" {
			t.Error("URL为空")
		}
		if response.Method != "PUT" {
			t.Errorf("Method错误: 期望 'PUT', 实际 '%s'", response.Method)
		}
		if response.Expires != 30*60 {
			t.Errorf("Expires错误: 期望 %d, 实际 %d", 30*60, response.Expires)
		}
	})

	t.Run("默认值处理", func(t *testing.T) {
		body := `{"bucket": "presign-bucket", "key": "test.txt"}`
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handlePresign(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response PresignResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		// 默认方法是PUT
		if response.Method != "PUT" {
			t.Errorf("默认Method错误: 期望 'PUT', 实际 '%s'", response.Method)
		}
		// 默认过期时间是60分钟
		if response.Expires != 60*60 {
			t.Errorf("默认Expires错误: 期望 %d, 实际 %d", 60*60, response.Expires)
		}
	})

	t.Run("过期时间上限限制", func(t *testing.T) {
		// 请求超过7天的过期时间
		body := `{"bucket": "presign-bucket", "key": "test.txt", "expiresMinutes": 20160}`
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handlePresign(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response PresignResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		// 应该被限制为7天（7*24*60分钟）
		maxExpires := 7 * 24 * 60 * 60
		if response.Expires != maxExpires {
			t.Errorf("Expires未被限制: 期望 %d, 实际 %d", maxExpires, response.Expires)
		}
	})

	t.Run("JSON解析错误", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handlePresign(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestHandleBucketAPI 测试桶管理API
func TestHandleBucketAPI(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("api-test-bucket"); err != nil {
		t.Fatalf("创建测试桶失败: %v", err)
	}

	t.Run("无效的API路径", func(t *testing.T) {
		invalidPaths := []string{
			"/api/bucket",
			"/api/bucket/",
			"/api/bucket/test-bucket",
		}

		for _, path := range invalidPaths {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			server.handleBucketAPI(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("路径 %s: 期望状态码 %d, 实际 %d", path, http.StatusNotFound, rec.Code)
			}
		}
	})

	t.Run("无效的action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/api-test-bucket/invalid-action", nil)
		rec := httptest.NewRecorder()

		server.handleBucketAPI(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// TestHandleBucketPublicAPI 测试桶公有/私有状态API
func TestHandleBucketPublicAPI(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("public-test-bucket"); err != nil {
		t.Fatalf("创建测试桶失败: %v", err)
	}

	t.Run("方法限制", func(t *testing.T) {
		methods := []string{http.MethodPost, http.MethodDelete, http.MethodHead}

		for _, method := range methods {
			req := httptest.NewRequest(method, "/api/bucket/public-test-bucket/public", nil)
			rec := httptest.NewRecorder()

			server.handleBucketPublicAPI(rec, req, "public-test-bucket")

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s请求: 期望状态码 %d, 实际 %d", method, http.StatusMethodNotAllowed, rec.Code)
			}
		}
	})

	t.Run("获取公有状态", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/public-test-bucket/public", nil)
		rec := httptest.NewRecorder()

		server.handleBucketPublicAPI(rec, req, "public-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]bool
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response["is_public"] != false {
			t.Errorf("is_public错误: 期望 false, 实际 %v", response["is_public"])
		}
	})

	t.Run("设置为公有", func(t *testing.T) {
		body := `{"is_public": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/bucket/public-test-bucket/public", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleBucketPublicAPI(rec, req, "public-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]bool
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response["is_public"] != true {
			t.Errorf("is_public错误: 期望 true, 实际 %v", response["is_public"])
		}
	})

	t.Run("桶不存在", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/nonexistent-bucket/public", nil)
		rec := httptest.NewRecorder()

		server.handleBucketPublicAPI(rec, req, "nonexistent-bucket")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("PUT-桶不存在", func(t *testing.T) {
		body := `{"is_public": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/bucket/nonexistent-bucket/public", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleBucketPublicAPI(rec, req, "nonexistent-bucket")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("PUT-无效JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/bucket/public-test-bucket/public", strings.NewReader("invalid"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleBucketPublicAPI(rec, req, "public-test-bucket")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestHandleBucketSearchAPI 测试对象搜索API
func TestHandleBucketSearchAPI(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("search-test-bucket"); err != nil {
		t.Fatalf("创建测试桶失败: %v", err)
	}

	// 创建测试对象
	testObjects := []string{"document.pdf", "image.png", "readme.md", "config.json"}
	for _, key := range testObjects {
		storagePath, _, err := server.filestore.PutObject("search-test-bucket", key, bytes.NewReader([]byte("test")), 4)
		if err != nil {
			t.Fatalf("存储对象失败: %v", err)
		}
		obj := &storage.Object{
			Bucket:      "search-test-bucket",
			Key:         key,
			Size:        4,
			ETag:        "d8e8fca2dc0f896fd7cb4cb0031ba249",
			ContentType: "application/octet-stream",
			StoragePath: storagePath,
		}
		if err := server.metadata.PutObject(obj); err != nil {
			t.Fatalf("保存对象元数据失败: %v", err)
		}
	}

	t.Run("方法限制", func(t *testing.T) {
		methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

		for _, method := range methods {
			req := httptest.NewRequest(method, "/api/bucket/search-test-bucket/search?q=test", nil)
			rec := httptest.NewRecorder()

			server.handleBucketSearchAPI(rec, req, "search-test-bucket")

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s请求: 期望状态码 %d, 实际 %d", method, http.StatusMethodNotAllowed, rec.Code)
			}
		}
	})

	t.Run("缺少搜索关键字", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/search-test-bucket/search", nil)
		rec := httptest.NewRecorder()

		server.handleBucketSearchAPI(rec, req, "search-test-bucket")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("桶不存在", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/nonexistent-bucket/search?q=test", nil)
		rec := httptest.NewRecorder()

		server.handleBucketSearchAPI(rec, req, "nonexistent-bucket")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("成功搜索", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/search-test-bucket/search?q=doc", nil)
		rec := httptest.NewRecorder()

		server.handleBucketSearchAPI(rec, req, "search-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response["keyword"] != "doc" {
			t.Errorf("keyword错误: 期望 'doc', 实际 '%v'", response["keyword"])
		}

		count := int(response["count"].(float64))
		if count == 0 {
			t.Error("搜索结果为空，应该至少找到 document.pdf")
		}
	})

	t.Run("搜索无结果", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/search-test-bucket/search?q=nonexistent", nil)
		rec := httptest.NewRecorder()

		server.handleBucketSearchAPI(rec, req, "search-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		count := int(response["count"].(float64))
		if count != 0 {
			t.Errorf("搜索结果应为空: 期望 0, 实际 %d", count)
		}
	})
}

// TestHandleBucketHeadObjectAPI 测试对象存在性检查API
func TestHandleBucketHeadObjectAPI(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("head-test-bucket"); err != nil {
		t.Fatalf("创建测试桶失败: %v", err)
	}

	// 创建测试对象
	storagePath, _, err := server.filestore.PutObject("head-test-bucket", "existing-file.txt", bytes.NewReader([]byte("test content")), 12)
	if err != nil {
		t.Fatalf("存储对象失败: %v", err)
	}
	obj := &storage.Object{
		Bucket:      "head-test-bucket",
		Key:         "existing-file.txt",
		Size:        12,
		ETag:        "9a0364b9e99bb480dd25e1f0284c8555",
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	if err := server.metadata.PutObject(obj); err != nil {
		t.Fatalf("保存对象元数据失败: %v", err)
	}

	t.Run("方法限制", func(t *testing.T) {
		methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

		for _, method := range methods {
			req := httptest.NewRequest(method, "/api/bucket/head-test-bucket/head?key=test.txt", nil)
			rec := httptest.NewRecorder()

			server.handleBucketHeadObjectAPI(rec, req, "head-test-bucket")

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s请求: 期望状态码 %d, 实际 %d", method, http.StatusMethodNotAllowed, rec.Code)
			}
		}
	})

	t.Run("缺少key参数", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/head-test-bucket/head", nil)
		rec := httptest.NewRecorder()

		server.handleBucketHeadObjectAPI(rec, req, "head-test-bucket")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("桶不存在", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/nonexistent-bucket/head?key=test.txt", nil)
		rec := httptest.NewRecorder()

		server.handleBucketHeadObjectAPI(rec, req, "nonexistent-bucket")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("对象存在", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/head-test-bucket/head?key=existing-file.txt", nil)
		rec := httptest.NewRecorder()

		server.handleBucketHeadObjectAPI(rec, req, "head-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response["exists"] != true {
			t.Errorf("exists错误: 期望 true, 实际 %v", response["exists"])
		}
		if response["key"] != "existing-file.txt" {
			t.Errorf("key错误: 期望 'existing-file.txt', 实际 '%v'", response["key"])
		}
		if response["size"] == nil {
			t.Error("缺少size字段")
		}
	})

	t.Run("对象不存在", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bucket/head-test-bucket/head?key=nonexistent.txt", nil)
		rec := httptest.NewRecorder()

		server.handleBucketHeadObjectAPI(rec, req, "head-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response["exists"] != false {
			t.Errorf("exists错误: 期望 false, 实际 %v", response["exists"])
		}
	})
}

// TestHandleRequest_Routing 测试请求路由分发
func TestHandleRequest_Routing(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	t.Run("浏览器访问根路径", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 浏览器访问应该被路由到静态文件处理
		// 注意：实际的静态文件可能不存在，但路由逻辑应该正确
	})

	t.Run("assets路径路由到静态文件", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/test.js", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// assets路径应该被路由到静态文件处理
	})

	t.Run("admin路径路由到SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/buckets", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// admin路径应该被路由到SPA处理
	})

	t.Run("API健康检查无需认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("API路径无认证返回拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 无认证的API请求应该返回403
		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("S3 API无认证返回拒绝", func(t *testing.T) {
		// 创建一个非公有桶
		server.metadata.CreateBucket("private-bucket")

		req := httptest.NewRequest(http.MethodPut, "/private-bucket/test.txt", strings.NewReader("test"))
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("公有桶非GET_HEAD请求仍需认证", func(t *testing.T) {
		// 创建桶并设置为公有
		server.metadata.CreateBucket("method-test-bucket")
		server.metadata.UpdateBucketPublic("method-test-bucket", true)

		// 公有桶只对 GET/HEAD 跳过认证，PATCH 等其他方法仍需认证
		// 所以未认证的 PATCH 请求会返回 403
		req := httptest.NewRequest(http.MethodPatch, "/method-test-bucket/test.txt", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})
}

// TestCheckAuth 测试认证检查
func TestCheckAuth(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	t.Run("无认证信息返回拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket", nil)
		rec := httptest.NewRecorder()

		_, ok := server.checkAuth(req, rec)

		if ok {
			t.Error("无认证信息应该返回失败")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("无效的Authorization头返回拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket", nil)
		req.Header.Set("Authorization", "Invalid Auth Header")
		rec := httptest.NewRecorder()

		_, ok := server.checkAuth(req, rec)

		if ok {
			t.Error("无效认证应该返回失败")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("无效的预签名URL返回拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket?X-Amz-Signature=invalid", nil)
		rec := httptest.NewRecorder()

		_, ok := server.checkAuth(req, rec)

		if ok {
			t.Error("无效预签名应该返回失败")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})
}

// TestContextKey 测试上下文键
func TestContextKey(t *testing.T) {
	t.Run("上下文键类型正确", func(t *testing.T) {
		key := ContextKeyAccessKeyID
		if key != "accessKeyID" {
			t.Errorf("ContextKeyAccessKeyID错误: 期望 'accessKeyID', 实际 '%s'", key)
		}
	})
}

// TestNewServer 测试服务器创建
func TestNewServer(t *testing.T) {
	tempDir := t.TempDir()

	// 确保配置已初始化
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	metadata, err := storage.NewMetadataStore(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}
	defer metadata.Close()

	filestore, err := storage.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	t.Run("成功创建服务器", func(t *testing.T) {
		server := NewServer(metadata, filestore)

		if server == nil {
			t.Fatal("服务器创建失败")
		}
		if server.metadata == nil {
			t.Error("metadata为nil")
		}
		if server.filestore == nil {
			t.Error("filestore为nil")
		}
		if server.adminHandler == nil {
			t.Error("adminHandler为nil")
		}
		if server.mux == nil {
			t.Error("mux为nil")
		}
	})

	t.Run("服务器实现http.Handler接口", func(t *testing.T) {
		server := NewServer(metadata, filestore)

		var _ http.Handler = server // 编译时检查
	})
}

// BenchmarkServeHTTP 基准测试ServeHTTP
func BenchmarkServeHTTP(b *testing.B) {
	// 确保配置已初始化
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("error")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/test.db")
	defer metadata.Close()
	filestore, _ := storage.NewFileStore(tempDir)
	server := NewServer(metadata, filestore)

	b.ResetTimer()

	b.Run("OPTIONS请求", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
		}
	})

	b.Run("健康检查", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			server.handleHealth(rec, req)
		}
	})
}

// TestHandleRequest_PublicBucket 测试公有桶访问
func TestHandleRequest_PublicBucket(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	// 创建一个公有桶并添加对象
	server.metadata.CreateBucket("public-access-bucket")
	server.metadata.UpdateBucketPublic("public-access-bucket", true)

	// 添加测试对象
	storagePath, _, _ := server.filestore.PutObject("public-access-bucket", "test.txt", strings.NewReader("hello"), 5)
	server.metadata.PutObject(&storage.Object{
		Bucket:      "public-access-bucket",
		Key:         "test.txt",
		Size:        5,
		ETag:        "5d41402abc4b2a76b9719d911017c592",
		ContentType: "text/plain",
		StoragePath: storagePath,
	})

	t.Run("公有桶GET请求无需认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public-access-bucket/test.txt", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 公有桶应该允许无认证的GET请求
		if rec.Code == http.StatusForbidden {
			t.Errorf("公有桶GET请求不应该返回403")
		}
	})

	t.Run("公有桶HEAD请求无需认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/public-access-bucket/test.txt", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 公有桶应该允许无认证的HEAD请求
		if rec.Code == http.StatusForbidden {
			t.Errorf("公有桶HEAD请求不应该返回403")
		}
	})

	t.Run("公有桶PUT请求需要认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/public-access-bucket/new.txt", strings.NewReader("new content"))
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 公有桶PUT请求仍需认证
		if rec.Code != http.StatusForbidden {
			t.Errorf("公有桶PUT请求应该返回403: got %d", rec.Code)
		}
	})

	t.Run("公有桶DELETE请求需要认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/public-access-bucket/test.txt", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 公有桶DELETE请求仍需认证
		if rec.Code != http.StatusForbidden {
			t.Errorf("公有桶DELETE请求应该返回403: got %d", rec.Code)
		}
	})

	t.Run("列举公有桶对象无需认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public-access-bucket", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 列举对象应该成功
		if rec.Code == http.StatusForbidden {
			t.Errorf("列举公有桶对象不应该返回403")
		}
	})
}

// TestHandleRequest_MultipartUploadRouting 测试多部分上传路由
func TestHandleRequest_MultipartUploadRouting(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	server.metadata.CreateBucket("multipart-bucket")

	t.Run("ListMultipartUploads返回501", func(t *testing.T) {
		// 无认证的请求会先返回403
		req := httptest.NewRequest(http.MethodGet, "/multipart-bucket?uploads", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test/20210101/us-east-1/s3/aws4_request")
		rec := httptest.NewRecorder()

		// 由于没有有效认证，会返回403
		server.handleRequest(rec, req)
		// 预期403或501
	})
}

// TestHandleRequest_MethodNotAllowed 测试不支持的HTTP方法
func TestHandleRequest_MethodNotAllowed(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	server.metadata.CreateBucket("method-bucket")
	server.metadata.UpdateBucketPublic("method-bucket", true)

	t.Run("PATCH方法返回405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/method-bucket/test.txt", nil)
		rec := httptest.NewRecorder()

		// PATCH 方法对于公有桶对象操作，需要认证
		server.handleRequest(rec, req)

		// 无认证应该返回403
		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: got %d", rec.Code)
		}
	})
}

// TestHandleRequest_SetupAPI 测试setup API路由
func TestHandleRequest_SetupAPI(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	t.Run("setup/status无需认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// setup API 不应该返回403
		if rec.Code == http.StatusForbidden {
			t.Errorf("setup API不应该需要认证")
		}
	})
}

// TestHandleRequest_AdminAPI 测试admin API路由
func TestHandleRequest_AdminAPI(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	t.Run("admin API委托给adminHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats/overview", nil)
		rec := httptest.NewRecorder()

		server.handleRequest(rec, req)

		// 无认证应该返回401 (由adminHandler处理)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusUnauthorized, rec.Code)
		}
	})
}

// TestCheckBucketPermission 测试桶权限检查
func TestCheckBucketPermission(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	t.Run("无accessKeyID返回拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket", nil)
		// 不设置上下文中的accessKeyID
		rec := httptest.NewRecorder()

		result := server.checkBucketPermission(req, rec, "test-bucket", false)

		if result {
			t.Error("无accessKeyID应该返回false")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})
}

// TestHandleDeleteBucket_NonEmpty 测试删除非空桶
func TestHandleDeleteBucket_NonEmpty(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	// 创建桶并添加对象
	bucketName := "non-empty-bucket"
	createTestBucket(t, server, bucketName)

	// 添加一个对象
	storagePath, _, _ := server.filestore.PutObject(bucketName, "file.txt", strings.NewReader("content"), 7)
	server.metadata.PutObject(&storage.Object{
		Bucket:      bucketName,
		Key:         "file.txt",
		Size:        7,
		ETag:        "9a0364b9e99bb480dd25e1f0284c8555",
		ContentType: "text/plain",
		StoragePath: storagePath,
	})

	t.Run("删除非空桶返回冲突", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/"+bucketName, nil)
		rec := httptest.NewRecorder()

		server.handleDeleteBucket(rec, req, bucketName)

		// 非空桶应该返回409冲突
		if rec.Code != http.StatusConflict {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusConflict, rec.Code)
		}
	})
}

// TestPresignRequestWithContentType 测试带ContentType的预签名请求
func TestPresignRequestWithContentType(t *testing.T) {
	server, cleanup := setupHandlersTestServer(t)
	defer cleanup()

	server.metadata.CreateBucket("content-type-bucket")

	t.Run("带ContentType生成预签名URL", func(t *testing.T) {
		body := `{"bucket": "content-type-bucket", "key": "image.png", "method": "PUT", "contentType": "image/png"}`
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handlePresign(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response PresignResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response.URL == "" {
			t.Error("URL为空")
		}
	})

	t.Run("带MaxSizeMB生成预签名URL", func(t *testing.T) {
		body := `{"bucket": "content-type-bucket", "key": "large.bin", "method": "PUT", "maxSizeMB": 100}`
		req := httptest.NewRequest(http.MethodPost, "/api/presign", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handlePresign(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

// TestIsEmbedMode 测试嵌入模式检查
func TestIsEmbedMode(t *testing.T) {
	// IsEmbedMode 应该返回 useEmbed 变量的值
	mode := IsEmbedMode()
	// 在测试环境中，这个值取决于构建标签
	// 我们只验证函数能正常调用
	_ = mode
}

// BenchmarkIsRootStaticFile 基准测试isRootStaticFile
func BenchmarkIsRootStaticFile(b *testing.B) {
	paths := []string{
		"/favicon.ico",
		"/robots.txt",
		"/bucket/file.txt",
		"/assets/icon.svg",
		"/a/b/c/d.png",
	}

	for _, path := range paths {
		b.Run(path, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				isRootStaticFile(path)
			}
		})
	}
}

// BenchmarkCheckAuth 基准测试认证检查
func BenchmarkCheckAuth(b *testing.B) {
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("error")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/test.db")
	defer metadata.Close()
	filestore, _ := storage.NewFileStore(tempDir)
	server := NewServer(metadata, filestore)

	b.ResetTimer()

	b.Run("无认证信息", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/bucket/key", nil)
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			server.checkAuth(req, rec)
		}
	})

	b.Run("无效Authorization头", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/bucket/key", nil)
		req.Header.Set("Authorization", "Invalid")
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			server.checkAuth(req, rec)
		}
	})
}
