package utils

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// ip.go 测试
// =============================================================================

// TestGetClientIP 测试获取客户端IP
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name:       "无代理头，使用RemoteAddr",
			headers:    nil,
			remoteAddr: "192.168.1.100:12345",
			want:       "192.168.1.100",
		},
		{
			name: "Cloudflare CF-Connecting-IP",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.50",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.50",
		},
		{
			name: "Nginx X-Real-IP",
			headers: map[string]string{
				"X-Real-IP": "198.51.100.25",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "198.51.100.25",
		},
		{
			name: "X-Forwarded-For 单个IP",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.100",
		},
		{
			name: "X-Forwarded-For 多个IP（取第一个）",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.100, 10.0.0.5, 10.0.0.1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.100",
		},
		{
			name: "True-Client-IP (Akamai)",
			headers: map[string]string{
				"True-Client-IP": "203.0.113.75",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.75",
		},
		{
			name: "Fastly-Client-IP",
			headers: map[string]string{
				"Fastly-Client-IP": "203.0.113.80",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.80",
		},
		{
			name: "X-Cluster-Client-IP (Rackspace)",
			headers: map[string]string{
				"X-Cluster-Client-IP": "203.0.113.85",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.85",
		},
		{
			name: "X-Client-IP",
			headers: map[string]string{
				"X-Client-IP": "203.0.113.90",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.90",
		},
		{
			name: "优先级测试：CF-Connecting-IP > X-Real-IP",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.50",
				"X-Real-IP":        "203.0.113.60",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.50",
		},
		{
			name: "无效IP头，回退到X-Forwarded-For",
			headers: map[string]string{
				"CF-Connecting-IP": "not-an-ip",
				"X-Forwarded-For":  "203.0.113.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.100",
		},
		{
			name: "无效IP头，回退到RemoteAddr",
			headers: map[string]string{
				"CF-Connecting-IP": "invalid",
				"X-Forwarded-For":  "also-invalid",
			},
			remoteAddr: "172.16.0.50:8080",
			want:       "172.16.0.50",
		},
		{
			name:       "IPv6 本地地址转换为 127.0.0.1",
			headers:    nil,
			remoteAddr: "[::1]:12345",
			want:       "127.0.0.1",
		},
		{
			name: "IPv6 公网地址",
			headers: map[string]string{
				"X-Real-IP": "2001:db8::1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "2001:db8::1",
		},
		{
			name:       "RemoteAddr 不含端口",
			headers:    nil,
			remoteAddr: "192.168.1.50",
			want:       "192.168.1.50",
		},
		{
			name: "头部值带空格",
			headers: map[string]string{
				"X-Real-IP": "  203.0.113.50  ",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := GetClientIP(req)
			if got != tt.want {
				t.Errorf("GetClientIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetUserAgent 测试获取 User-Agent
func TestGetUserAgent(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		wantLen   int
	}{
		{
			name:      "正常 User-Agent",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			wantLen:   -1, // -1 表示不限制，取原长度
		},
		{
			name:      "空 User-Agent",
			userAgent: "",
			wantLen:   0,
		},
		{
			name:      "超长 User-Agent 截断到 500",
			userAgent: strings.Repeat("a", 1000),
			wantLen:   500,
		},
		{
			name:      "刚好 500 字符",
			userAgent: strings.Repeat("b", 500),
			wantLen:   500,
		},
		{
			name:      "短 User-Agent",
			userAgent: "curl/7.68.0",
			wantLen:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("User-Agent", tt.userAgent)

			got := GetUserAgent(req)

			expectedLen := tt.wantLen
			if expectedLen == -1 {
				expectedLen = len(tt.userAgent)
			}

			if len(got) != expectedLen {
				t.Errorf("GetUserAgent() 长度 = %v, want %v", len(got), expectedLen)
			}

			// 检查截断后的内容
			if tt.wantLen == 500 && got != tt.userAgent[:500] {
				t.Errorf("GetUserAgent() 截断不正确")
			}
		})
	}
}

// TestIsPrivateIP 测试私有IP判断
func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		// 10.0.0.0/8 私有地址
		{"10.0.0.1", "10.0.0.1", true},
		{"10.255.255.255", "10.255.255.255", true},
		{"10.100.50.25", "10.100.50.25", true},

		// 172.16.0.0/12 私有地址
		{"172.16.0.1", "172.16.0.1", true},
		{"172.31.255.255", "172.31.255.255", true},
		{"172.20.100.50", "172.20.100.50", true},
		// 边界外
		{"172.15.255.255", "172.15.255.255", false},
		{"172.32.0.1", "172.32.0.1", false},

		// 192.168.0.0/16 私有地址
		{"192.168.0.1", "192.168.0.1", true},
		{"192.168.255.255", "192.168.255.255", true},
		{"192.168.100.100", "192.168.100.100", true},

		// 127.0.0.0/8 本地回环
		{"127.0.0.1", "127.0.0.1", true},
		{"127.255.255.255", "127.255.255.255", true},

		// IPv6 本地回环
		{"::1", "::1", true},

		// IPv6 私有地址 (fc00::/7)
		{"fc00::1", "fc00::1", true},
		{"fd00::1", "fd00::1", true},

		// 公网 IP
		{"8.8.8.8", "8.8.8.8", false},
		{"203.0.113.50", "203.0.113.50", false},
		{"1.1.1.1", "1.1.1.1", false},

		// IPv6 公网地址
		{"2001:db8::1", "2001:db8::1", false},

		// 无效 IP
		{"invalid", "invalid", false},
		{"", "", false},
		{"256.256.256.256", "256.256.256.256", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPrivateIP(tt.ip)
			if got != tt.want {
				t.Errorf("IsPrivateIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// =============================================================================
// logger.go 测试
// =============================================================================

// TestInitLogger 测试日志初始化
func TestInitLogger(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug级别", "debug"},
		{"info级别", "info"},
		{"warn级别", "warn"},
		{"error级别", "error"},
		{"默认级别（未知值）", "unknown"},
		{"空级别", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 调用初始化，不应该 panic
			InitLogger(tt.level)

			if Logger == nil {
				t.Error("InitLogger() 后 Logger 不应为 nil")
			}
		})
	}
}

// TestLoggerFunctions 测试日志函数
func TestLoggerFunctions(t *testing.T) {
	// 先初始化
	InitLogger("debug")

	// 测试各个日志函数不会 panic
	t.Run("Info", func(t *testing.T) {
		Info("测试 info 日志", "key", "value")
	})

	t.Run("Debug", func(t *testing.T) {
		Debug("测试 debug 日志", "key", "value")
	})

	t.Run("Warn", func(t *testing.T) {
		Warn("测试 warn 日志", "key", "value")
	})

	t.Run("Error", func(t *testing.T) {
		Error("测试 error 日志", "key", "value")
	})

	t.Run("带多个参数", func(t *testing.T) {
		Info("多参数日志", "key1", "value1", "key2", 123, "key3", true)
	})
}

// =============================================================================
// response.go 测试
// =============================================================================

// TestWriteError 测试写入 S3 错误响应
func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        S3Error
		statusCode int
		resource   string
	}{
		{
			name:       "NoSuchBucket 错误",
			err:        ErrNoSuchBucket,
			statusCode: http.StatusNotFound,
			resource:   "/my-bucket",
		},
		{
			name:       "NoSuchKey 错误",
			err:        ErrNoSuchKey,
			statusCode: http.StatusNotFound,
			resource:   "/my-bucket/my-key",
		},
		{
			name:       "AccessDenied 错误",
			err:        ErrAccessDenied,
			statusCode: http.StatusForbidden,
			resource:   "/private-bucket",
		},
		{
			name:       "InternalError 错误",
			err:        ErrInternalError,
			statusCode: http.StatusInternalServerError,
			resource:   "",
		},
		{
			name:       "SignatureDoesNotMatch 错误",
			err:        ErrSignatureDoesNotMatch,
			statusCode: http.StatusForbidden,
			resource:   "/bucket/key",
		},
		{
			name:       "EntityTooLarge 错误",
			err:        ErrEntityTooLarge,
			statusCode: http.StatusRequestEntityTooLarge,
			resource:   "/upload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tt.err, tt.statusCode, tt.resource)

			// 检查状态码
			if w.Code != tt.statusCode {
				t.Errorf("WriteError() 状态码 = %v, want %v", w.Code, tt.statusCode)
			}

			// 检查 Content-Type
			if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
				t.Errorf("Content-Type = %v, want application/xml", ct)
			}

			// 解析响应
			var respErr S3Error
			if err := xml.Unmarshal(w.Body.Bytes(), &respErr); err != nil {
				t.Errorf("解析 XML 失败: %v", err)
				return
			}

			// 检查错误码
			if respErr.Code != tt.err.Code {
				t.Errorf("错误码 = %v, want %v", respErr.Code, tt.err.Code)
			}

			// 检查资源
			if respErr.Resource != tt.resource {
				t.Errorf("Resource = %v, want %v", respErr.Resource, tt.resource)
			}

			// 检查 RequestID 已生成
			if respErr.RequestID == "" {
				t.Error("RequestID 不应为空")
			}
		})
	}
}

// TestWriteXML 测试写入 XML 响应
func TestWriteXML(t *testing.T) {
	type TestStruct struct {
		XMLName xml.Name `xml:"TestResponse"`
		Name    string   `xml:"Name"`
		Value   int      `xml:"Value"`
	}

	tests := []struct {
		name       string
		statusCode int
		data       interface{}
	}{
		{
			name:       "简单结构体",
			statusCode: http.StatusOK,
			data: TestStruct{
				Name:  "test",
				Value: 123,
			},
		},
		{
			name:       "创建成功 (201)",
			statusCode: http.StatusCreated,
			data: TestStruct{
				Name:  "created",
				Value: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteXML(w, tt.statusCode, tt.data)

			// 检查状态码
			if w.Code != tt.statusCode {
				t.Errorf("WriteXML() 状态码 = %v, want %v", w.Code, tt.statusCode)
			}

			// 检查 Content-Type
			if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
				t.Errorf("Content-Type = %v, want application/xml", ct)
			}

			// 检查包含 XML 头
			body := w.Body.String()
			if !strings.Contains(body, "<?xml version=") {
				t.Error("响应应包含 XML 声明头")
			}

			// 尝试解析
			var resp TestStruct
			// 去掉 XML 头后解析
			xmlContent := strings.TrimPrefix(body, xml.Header)
			if err := xml.Unmarshal([]byte(xmlContent), &resp); err != nil {
				t.Errorf("解析 XML 失败: %v", err)
			}
		})
	}
}

// TestGenerateRequestID 测试生成请求ID
func TestGenerateRequestID(t *testing.T) {
	t.Run("生成唯一ID", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := GenerateRequestID()
			if ids[id] {
				t.Errorf("GenerateRequestID() 生成了重复 ID: %s", id)
			}
			ids[id] = true
		}
	})

	t.Run("ID长度正确", func(t *testing.T) {
		id := GenerateRequestID()
		// 16 字节的 hex 编码 = 32 字符
		if len(id) != 32 {
			t.Errorf("GenerateRequestID() 长度 = %d, want 32", len(id))
		}
	})

	t.Run("ID只包含十六进制字符", func(t *testing.T) {
		id := GenerateRequestID()
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("GenerateRequestID() 包含非法字符: %c", c)
			}
		}
	})
}

// TestParseJSONBody 测试解析 JSON 请求体
func TestParseJSONBody(t *testing.T) {
	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "有效 JSON",
			body:    `{"name":"test","value":123}`,
			wantErr: false,
		},
		{
			name:    "无效 JSON",
			body:    `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "空 JSON 对象",
			body:    `{}`,
			wantErr: false,
		},
		{
			name:    "空请求体",
			body:    ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			var data TestData
			err := ParseJSONBody(req, &data)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSONBody() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestWriteJSONResponse 测试写入 JSON 响应
func TestWriteJSONResponse(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{
			name: "简单对象",
			data: map[string]interface{}{
				"success": true,
				"message": "OK",
			},
		},
		{
			name: "结构体",
			data: struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{
				Name:  "test",
				Value: 123,
			},
		},
		{
			name: "数组",
			data: []string{"a", "b", "c"},
		},
		{
			name: "嵌套对象",
			data: map[string]interface{}{
				"user": map[string]string{
					"name":  "张三",
					"email": "test@example.com",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteJSONResponse(w, tt.data)

			// 检查 Content-Type
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %v, want application/json", ct)
			}

			// 尝试解析响应
			var result interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Errorf("解析 JSON 失败: %v", err)
			}
		})
	}
}

// TestWriteErrorResponse 测试写入 JSON 错误响应
func TestWriteErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		message    string
		statusCode int
	}{
		{
			name:       "400 错误请求",
			code:       "InvalidRequest",
			message:    "请求参数无效",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "401 未授权",
			code:       "Unauthorized",
			message:    "认证失败",
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "403 禁止访问",
			code:       "Forbidden",
			message:    "没有权限",
			statusCode: http.StatusForbidden,
		},
		{
			name:       "404 未找到",
			code:       "NotFound",
			message:    "资源不存在",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "500 内部错误",
			code:       "InternalError",
			message:    "服务器内部错误",
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteErrorResponse(w, tt.code, tt.message, tt.statusCode)

			// 检查状态码
			if w.Code != tt.statusCode {
				t.Errorf("WriteErrorResponse() 状态码 = %v, want %v", w.Code, tt.statusCode)
			}

			// 检查 Content-Type
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %v, want application/json", ct)
			}

			// 解析响应
			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Errorf("解析 JSON 失败: %v", err)
				return
			}

			// 检查错误码
			if resp["error"] != tt.code {
				t.Errorf("error = %v, want %v", resp["error"], tt.code)
			}

			// 检查消息
			if resp["message"] != tt.message {
				t.Errorf("message = %v, want %v", resp["message"], tt.message)
			}
		})
	}
}

// TestPreDefinedErrors 测试预定义错误
func TestPreDefinedErrors(t *testing.T) {
	errors := []struct {
		name    string
		err     S3Error
		wantCode string
	}{
		{"ErrNoSuchBucket", ErrNoSuchBucket, "NoSuchBucket"},
		{"ErrNoSuchKey", ErrNoSuchKey, "NoSuchKey"},
		{"ErrBucketAlreadyExists", ErrBucketAlreadyExists, "BucketAlreadyExists"},
		{"ErrBucketNotEmpty", ErrBucketNotEmpty, "BucketNotEmpty"},
		{"ErrAccessDenied", ErrAccessDenied, "AccessDenied"},
		{"ErrSignatureDoesNotMatch", ErrSignatureDoesNotMatch, "SignatureDoesNotMatch"},
		{"ErrInvalidAccessKeyId", ErrInvalidAccessKeyId, "InvalidAccessKeyId"},
		{"ErrNoSuchUpload", ErrNoSuchUpload, "NoSuchUpload"},
		{"ErrInvalidPart", ErrInvalidPart, "InvalidPart"},
		{"ErrInvalidArgument", ErrInvalidArgument, "InvalidArgument"},
		{"ErrInternalError", ErrInternalError, "InternalError"},
		{"ErrMethodNotAllowed", ErrMethodNotAllowed, "MethodNotAllowed"},
		{"ErrMalformedJSON", ErrMalformedJSON, "MalformedJSON"},
		{"ErrEntityTooLarge", ErrEntityTooLarge, "EntityTooLarge"},
		{"ErrBadDigest", ErrBadDigest, "BadDigest"},
	}

	for _, tt := range errors {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("%s.Code = %v, want %v", tt.name, tt.err.Code, tt.wantCode)
			}
			if tt.err.Message == "" {
				t.Errorf("%s.Message 不应为空", tt.name)
			}
		})
	}
}

// =============================================================================
// util.go 测试
// =============================================================================

// TestGenerateID 测试生成随机ID
func TestGenerateID(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"16字节", 16},
		{"8字节", 8},
		{"32字节", 32},
		{"1字节", 1},
		{"0字节", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.length)

			// 检查长度（hex编码后是原长度的2倍）
			expectedLen := tt.length * 2
			if len(id) != expectedLen {
				t.Errorf("GenerateID(%d) 长度 = %d, want %d", tt.length, len(id), expectedLen)
			}

			// 检查只包含十六进制字符
			for _, c := range id {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("GenerateID() 包含非法字符: %c", c)
				}
			}
		})
	}
}

// TestGenerateIDUniqueness 测试ID唯一性
func TestGenerateIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	const count = 1000

	for i := 0; i < count; i++ {
		id := GenerateID(16)
		if ids[id] {
			t.Errorf("GenerateID() 在 %d 次调用中生成了重复 ID", count)
		}
		ids[id] = true
	}
}

// BenchmarkGenerateID 基准测试ID生成
func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateID(16)
	}
}

// BenchmarkGenerateRequestID 基准测试请求ID生成
func BenchmarkGenerateRequestID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateRequestID()
	}
}

// BenchmarkGetClientIP 基准测试获取客户端IP
func BenchmarkGetClientIP(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.100, 10.0.0.1")
	req.RemoteAddr = "10.0.0.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetClientIP(req)
	}
}

// BenchmarkIsPrivateIP 基准测试私有IP判断
func BenchmarkIsPrivateIP(b *testing.B) {
	ips := []string{"192.168.1.1", "10.0.0.1", "8.8.8.8", "172.16.0.1", "203.0.113.50"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ip := range ips {
			IsPrivateIP(ip)
		}
	}
}

// BenchmarkWriteJSONResponse 基准测试JSON响应写入
func BenchmarkWriteJSONResponse(b *testing.B) {
	data := map[string]interface{}{
		"success": true,
		"message": "OK",
		"data": map[string]string{
			"name": "test",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		WriteJSONResponse(w, data)
	}
}

// BenchmarkWriteError 基准测试S3错误响应写入
func BenchmarkWriteError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		WriteError(w, ErrNoSuchBucket, http.StatusNotFound, "/my-bucket")
	}
}
