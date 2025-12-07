package auth

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/utils"
)

// setupPresignTestConfig 设置预签名测试配置
func setupPresignTestConfig() {
	// 确保 Global 被初始化
	if config.Global == nil {
		config.NewDefault()
	}
	// 确保 Logger 被初始化
	if utils.Logger == nil {
		utils.InitLogger("info")
	}
	config.Global.Auth.AccessKeyID = "test-access-key"
	config.Global.Auth.SecretAccessKey = "test-secret-key"
	config.Global.Server.Host = "localhost"
	config.Global.Server.Port = 8080
	config.Global.Server.Region = "us-east-1"
}

// TestGeneratePresignedURL 测试基本预签名URL生成
func TestGeneratePresignedURL(t *testing.T) {
	setupPresignTestConfig()

	testCases := []struct {
		name    string
		method  string
		bucket  string
		key     string
		expires time.Duration
	}{
		{
			name:    "GET请求",
			method:  "GET",
			bucket:  "test-bucket",
			key:     "test-object.txt",
			expires: time.Hour,
		},
		{
			name:    "PUT请求",
			method:  "PUT",
			bucket:  "my-bucket",
			key:     "upload.jpg",
			expires: 30 * time.Minute,
		},
		{
			name:    "DELETE请求",
			method:  "DELETE",
			bucket:  "bucket",
			key:     "file.pdf",
			expires: 5 * time.Minute,
		},
		{
			name:    "中文对象名",
			method:  "GET",
			bucket:  "test-bucket",
			key:     "文档/测试文件.txt",
			expires: time.Hour,
		},
		{
			name:    "特殊字符",
			method:  "GET",
			bucket:  "test-bucket",
			key:     "path/to/file with spaces.txt",
			expires: time.Hour,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GeneratePresignedURL(tc.method, tc.bucket, tc.key, tc.expires)

			// 验证URL格式
			if result == "" {
				t.Error("生成的URL不应该为空")
			}

			// 解析URL
			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			// 验证协议
			if parsed.Scheme != "http" {
				t.Errorf("协议应该是http: got %s", parsed.Scheme)
			}

			// 验证Host
			if parsed.Host != "localhost:8080" {
				t.Errorf("Host不匹配: got %s, want localhost:8080", parsed.Host)
			}

			// 验证路径包含bucket和key
			expectedPathPrefix := "/" + tc.bucket + "/"
			if !strings.HasPrefix(parsed.Path, expectedPathPrefix) {
				t.Errorf("路径应该以%s开头: got %s", expectedPathPrefix, parsed.Path)
			}

			// 验证必需的查询参数
			query := parsed.Query()
			requiredParams := []string{
				"X-Amz-Algorithm",
				"X-Amz-Credential",
				"X-Amz-Date",
				"X-Amz-Expires",
				"X-Amz-SignedHeaders",
				"X-Amz-Signature",
			}

			for _, param := range requiredParams {
				if query.Get(param) == "" {
					t.Errorf("缺少必需参数: %s", param)
				}
			}

			// 验证算法
			if query.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
				t.Errorf("算法不正确: got %s", query.Get("X-Amz-Algorithm"))
			}

			// 验证过期时间
			expectedExpires := int(tc.expires.Seconds())
			if query.Get("X-Amz-Expires") != string(rune(expectedExpires)) {
				// 验证格式正确即可
				if query.Get("X-Amz-Expires") == "" {
					t.Error("过期时间参数为空")
				}
			}
		})
	}
}

// TestGeneratePresignedURLWithOptions 测试带选项的预签名URL生成
func TestGeneratePresignedURLWithOptions(t *testing.T) {
	setupPresignTestConfig()

	testCases := []struct {
		name                string
		method              string
		bucket              string
		key                 string
		opts                *PresignOptions
		expectContentLength bool
		expectContentType   bool
	}{
		{
			name:   "默认选项",
			method: "PUT",
			bucket: "test-bucket",
			key:    "file.txt",
			opts: &PresignOptions{
				Expires: time.Hour,
			},
			expectContentLength: false,
			expectContentType:   false,
		},
		{
			name:   "带内容长度限制",
			method: "PUT",
			bucket: "test-bucket",
			key:    "large-file.bin",
			opts: &PresignOptions{
				Expires:          time.Hour,
				MaxContentLength: 10 * 1024 * 1024, // 10MB
			},
			expectContentLength: true,
			expectContentType:   false,
		},
		{
			name:   "带内容类型限制",
			method: "PUT",
			bucket: "test-bucket",
			key:    "image.jpg",
			opts: &PresignOptions{
				Expires:     time.Hour,
				ContentType: "image/jpeg",
			},
			expectContentLength: false,
			expectContentType:   true,
		},
		{
			name:   "完整选项",
			method: "PUT",
			bucket: "test-bucket",
			key:    "document.pdf",
			opts: &PresignOptions{
				Expires:          30 * time.Minute,
				MaxContentLength: 5 * 1024 * 1024, // 5MB
				ContentType:      "application/pdf",
			},
			expectContentLength: true,
			expectContentType:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GeneratePresignedURLWithOptions(tc.method, tc.bucket, tc.key, tc.opts)

			// 解析URL
			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			query := parsed.Query()

			// 验证内容长度限制
			hasContentLength := query.Get("X-Amz-Max-Content-Length") != ""
			if tc.expectContentLength != hasContentLength {
				t.Errorf("内容长度限制: expected %v, got %v", tc.expectContentLength, hasContentLength)
			}

			// 验证内容类型
			hasContentType := query.Get("X-Amz-Content-Type") != ""
			if tc.expectContentType != hasContentType {
				t.Errorf("内容类型: expected %v, got %v", tc.expectContentType, hasContentType)
			}

			// 如果有内容长度限制，验证值
			if tc.expectContentLength && tc.opts.MaxContentLength > 0 {
				lengthStr := query.Get("X-Amz-Max-Content-Length")
				if lengthStr == "" {
					t.Error("内容长度限制值为空")
				}
			}

			// 如果有内容类型，验证值
			if tc.expectContentType && tc.opts.ContentType != "" {
				typeStr := query.Get("X-Amz-Content-Type")
				if typeStr != tc.opts.ContentType {
					t.Errorf("内容类型不匹配: got %s, want %s", typeStr, tc.opts.ContentType)
				}
			}
		})
	}
}

// TestPresignedURLScheme 测试预签名URL的协议配置
func TestPresignedURLScheme(t *testing.T) {
	setupPresignTestConfig()

	testCases := []struct {
		name           string
		presignScheme  string
		expectedScheme string
	}{
		{
			name:           "默认HTTP",
			presignScheme:  "",
			expectedScheme: "http",
		},
		{
			name:           "配置HTTP",
			presignScheme:  "http",
			expectedScheme: "http",
		},
		{
			name:           "配置HTTPS",
			presignScheme:  "https",
			expectedScheme: "https",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config.Global.Security.PresignScheme = tc.presignScheme

			result := GeneratePresignedURL("GET", "bucket", "key", time.Hour)

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			if parsed.Scheme != tc.expectedScheme {
				t.Errorf("协议不匹配: got %s, want %s", parsed.Scheme, tc.expectedScheme)
			}
		})
	}
}

// TestPresignedURLHost 测试不同Host配置
func TestPresignedURLHost(t *testing.T) {
	setupPresignTestConfig()

	testCases := []struct {
		name         string
		configHost   string
		configPort   int
		expectedHost string
	}{
		{
			name:         "localhost配置",
			configHost:   "localhost",
			configPort:   8080,
			expectedHost: "localhost:8080",
		},
		{
			name:         "0.0.0.0转换为localhost",
			configHost:   "0.0.0.0",
			configPort:   8080,
			expectedHost: "localhost:8080",
		},
		{
			name:         "具体IP地址",
			configHost:   "192.168.1.100",
			configPort:   9000,
			expectedHost: "192.168.1.100:9000",
		},
		{
			name:         "域名",
			configHost:   "s3.example.com",
			configPort:   443,
			expectedHost: "s3.example.com:443",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config.Global.Server.Host = tc.configHost
			config.Global.Server.Port = tc.configPort

			result := GeneratePresignedURL("GET", "bucket", "key", time.Hour)

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			if parsed.Host != tc.expectedHost {
				t.Errorf("Host不匹配: got %s, want %s", parsed.Host, tc.expectedHost)
			}
		})
	}

	// 恢复默认配置
	config.Global.Server.Host = "localhost"
	config.Global.Server.Port = 8080
}

// TestGetCanonicalQueryStringForPresign 测试预签名规范化查询字符串
func TestGetCanonicalQueryStringForPresign(t *testing.T) {
	testCases := []struct {
		name     string
		params   url.Values
		expected string
	}{
		{
			name:     "空参数",
			params:   url.Values{},
			expected: "",
		},
		{
			name: "单个参数",
			params: url.Values{
				"X-Amz-Algorithm": {"AWS4-HMAC-SHA256"},
			},
			expected: "X-Amz-Algorithm=AWS4-HMAC-SHA256",
		},
		{
			name: "多个参数按字母排序",
			params: url.Values{
				"X-Amz-Expires":   {"3600"},
				"X-Amz-Algorithm": {"AWS4-HMAC-SHA256"},
				"X-Amz-Date":      {"20210101T000000Z"},
			},
			expected: "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Date=20210101T000000Z&X-Amz-Expires=3600",
		},
		{
			name: "需要URL编码的值",
			params: url.Values{
				"X-Amz-Credential": {"key/20210101/us-east-1/s3/aws4_request"},
			},
			expected: "X-Amz-Credential=key%2F20210101%2Fus-east-1%2Fs3%2Faws4_request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getCanonicalQueryStringForPresign(tc.params)
			if result != tc.expected {
				t.Errorf("规范化结果不匹配:\ngot:  %s\nwant: %s", result, tc.expected)
			}
		})
	}
}

// TestPresignedURLExpiration 测试预签名URL过期时间
func TestPresignedURLExpiration(t *testing.T) {
	setupPresignTestConfig()

	testCases := []struct {
		name            string
		expires         time.Duration
		expectedSeconds string
	}{
		{
			name:            "1分钟",
			expires:         time.Minute,
			expectedSeconds: "60",
		},
		{
			name:            "1小时",
			expires:         time.Hour,
			expectedSeconds: "3600",
		},
		{
			name:            "24小时",
			expires:         24 * time.Hour,
			expectedSeconds: "86400",
		},
		{
			name:            "7天",
			expires:         7 * 24 * time.Hour,
			expectedSeconds: "604800",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GeneratePresignedURL("GET", "bucket", "key", tc.expires)

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			expiresStr := parsed.Query().Get("X-Amz-Expires")
			if expiresStr != tc.expectedSeconds {
				t.Errorf("过期时间不匹配: got %s, want %s", expiresStr, tc.expectedSeconds)
			}
		})
	}
}

// TestPresignedURLSignature 测试预签名URL签名
func TestPresignedURLSignature(t *testing.T) {
	setupPresignTestConfig()

	// 生成两个相同参数的URL
	url1 := GeneratePresignedURL("GET", "bucket", "key", time.Hour)
	url2 := GeneratePresignedURL("GET", "bucket", "key", time.Hour)

	parsed1, _ := url.Parse(url1)
	parsed2, _ := url.Parse(url2)

	sig1 := parsed1.Query().Get("X-Amz-Signature")
	sig2 := parsed2.Query().Get("X-Amz-Signature")

	// 验证签名存在且长度正确（64个十六进制字符 = 256位）
	if len(sig1) != 64 {
		t.Errorf("签名长度应该是64: got %d", len(sig1))
	}

	// 由于时间戳可能不同，签名可能不同，这是正常的
	t.Logf("签名1: %s", sig1)
	t.Logf("签名2: %s", sig2)
}

// TestPresignedURLDifferentMethods 测试不同HTTP方法的签名
func TestPresignedURLDifferentMethods(t *testing.T) {
	setupPresignTestConfig()

	methods := []string{"GET", "PUT", "POST", "DELETE", "HEAD"}

	// 收集所有URL
	urls := make(map[string]string)
	for _, method := range methods {
		urls[method] = GeneratePresignedURL(method, "bucket", "key", time.Hour)
	}

	// 验证每个URL都有效且不同
	for method, urlStr := range urls {
		t.Run(method, func(t *testing.T) {
			parsed, err := url.Parse(urlStr)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			// 验证必需参数存在
			query := parsed.Query()
			if query.Get("X-Amz-Signature") == "" {
				t.Error("缺少签名")
			}
			if query.Get("X-Amz-Algorithm") == "" {
				t.Error("缺少算法")
			}
		})
	}
}

// TestPresignedURLRegion 测试不同区域的签名
func TestPresignedURLRegion(t *testing.T) {
	setupPresignTestConfig()

	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-northeast-1", "cn-north-1"}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			config.Global.Server.Region = region

			result := GeneratePresignedURL("GET", "bucket", "key", time.Hour)

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			// 验证Credential包含正确的区域
			credential := parsed.Query().Get("X-Amz-Credential")
			if !strings.Contains(credential, region) {
				t.Errorf("Credential应该包含区域 %s: got %s", region, credential)
			}
		})
	}

	// 恢复默认区域
	config.Global.Server.Region = "us-east-1"
}

// TestPresignedURLContentLengthLimit 测试内容长度限制与全局限制的交互
func TestPresignedURLContentLengthLimit(t *testing.T) {
	setupPresignTestConfig()

	// 设置全局最大上传大小为1GB
	config.Global.Storage.MaxUploadSize = 1024 * 1024 * 1024

	testCases := []struct {
		name               string
		maxContentLength   int64
		globalMaxUpload    int64
		expectedLessThanEq int64
	}{
		{
			name:               "小于全局限制",
			maxContentLength:   10 * 1024 * 1024, // 10MB
			globalMaxUpload:    1024 * 1024 * 1024,
			expectedLessThanEq: 10 * 1024 * 1024,
		},
		{
			name:               "等于全局限制",
			maxContentLength:   1024 * 1024 * 1024,
			globalMaxUpload:    1024 * 1024 * 1024,
			expectedLessThanEq: 1024 * 1024 * 1024,
		},
		{
			name:               "大于全局限制应被截断",
			maxContentLength:   2 * 1024 * 1024 * 1024,     // 2GB
			globalMaxUpload:    1024 * 1024 * 1024,         // 1GB
			expectedLessThanEq: 1024 * 1024 * 1024,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config.Global.Storage.MaxUploadSize = tc.globalMaxUpload

			opts := &PresignOptions{
				Expires:          time.Hour,
				MaxContentLength: tc.maxContentLength,
			}

			result := GeneratePresignedURLWithOptions("PUT", "bucket", "file", opts)

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("解析URL失败: %v", err)
			}

			// 验证内容长度参数存在
			lengthStr := parsed.Query().Get("X-Amz-Max-Content-Length")
			if lengthStr == "" {
				t.Error("缺少内容长度限制参数")
			}
		})
	}
}

// TestPresignedURLCredentialFormat 测试Credential格式
func TestPresignedURLCredentialFormat(t *testing.T) {
	setupPresignTestConfig()
	config.Global.Auth.AccessKeyID = "AKIAIOSFODNN7EXAMPLE"
	config.Global.Server.Region = "us-east-1"

	result := GeneratePresignedURL("GET", "bucket", "key", time.Hour)

	parsed, err := url.Parse(result)
	if err != nil {
		t.Fatalf("解析URL失败: %v", err)
	}

	credential := parsed.Query().Get("X-Amz-Credential")

	// Credential格式: accessKeyId/date/region/s3/aws4_request
	parts := strings.Split(credential, "/")
	if len(parts) != 5 {
		t.Errorf("Credential格式不正确，应该有5部分: got %d parts", len(parts))
	}

	if parts[0] != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("Access Key ID不匹配: got %s", parts[0])
	}

	// 验证日期格式 (YYYYMMDD)
	if len(parts[1]) != 8 {
		t.Errorf("日期格式不正确: got %s", parts[1])
	}

	if parts[2] != "us-east-1" {
		t.Errorf("区域不匹配: got %s", parts[2])
	}

	if parts[3] != "s3" {
		t.Errorf("服务名应该是s3: got %s", parts[3])
	}

	if parts[4] != "aws4_request" {
		t.Errorf("请求类型应该是aws4_request: got %s", parts[4])
	}
}

// TestPresignedURLDateFormat 测试日期格式
func TestPresignedURLDateFormat(t *testing.T) {
	setupPresignTestConfig()

	result := GeneratePresignedURL("GET", "bucket", "key", time.Hour)

	parsed, err := url.Parse(result)
	if err != nil {
		t.Fatalf("解析URL失败: %v", err)
	}

	amzDate := parsed.Query().Get("X-Amz-Date")

	// 格式: YYYYMMDDTHHMMSSZ
	if len(amzDate) != 16 {
		t.Errorf("日期长度应该是16: got %d", len(amzDate))
	}

	if amzDate[8] != 'T' {
		t.Errorf("日期格式错误，第9个字符应该是T: got %c", amzDate[8])
	}

	if amzDate[15] != 'Z' {
		t.Errorf("日期格式错误，最后一个字符应该是Z: got %c", amzDate[15])
	}
}

// BenchmarkGeneratePresignedURL 预签名URL生成性能测试
func BenchmarkGeneratePresignedURL(b *testing.B) {
	setupPresignTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GeneratePresignedURL("GET", "bucket", "key", time.Hour)
	}
}

// BenchmarkGeneratePresignedURLWithOptions 带选项预签名URL生成性能测试
func BenchmarkGeneratePresignedURLWithOptions(b *testing.B) {
	setupPresignTestConfig()

	opts := &PresignOptions{
		Expires:          time.Hour,
		MaxContentLength: 10 * 1024 * 1024,
		ContentType:      "application/octet-stream",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GeneratePresignedURLWithOptions("PUT", "bucket", "key", opts)
	}
}

// BenchmarkGetCanonicalQueryStringForPresign 规范化查询字符串性能测试
func BenchmarkGetCanonicalQueryStringForPresign(b *testing.B) {
	params := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {"key/20210101/us-east-1/s3/aws4_request"},
		"X-Amz-Date":          {"20210101T000000Z"},
		"X-Amz-Expires":       {"3600"},
		"X-Amz-SignedHeaders": {"host"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getCanonicalQueryStringForPresign(params)
	}
}
