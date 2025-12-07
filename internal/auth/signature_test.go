package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// setupTestConfig 设置测试配置
func setupTestConfig() {
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
}

// setupTestStore 创建测试用的元数据存储
func setupTestStore(t *testing.T) (*storage.MetadataStore, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	store, err := storage.NewMetadataStore(dbPath)
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}
	return store, func() { store.Close() }
}

// TestHmacSHA256 测试HMAC-SHA256计算
func TestHmacSHA256(t *testing.T) {
	testCases := []struct {
		name     string
		key      string
		data     string
		expected string
	}{
		{
			name:     "简单测试",
			key:      "key",
			data:     "data",
			expected: "5031fe3d989c6d1537a013fa6e739da23463fdaec3b70137d828e36ace221bd0",
		},
		{
			name:     "AWS测试向量",
			key:      "AWS4wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
			data:     "20150830",
			expected: "0138c7a6cbd60aa727b2f653a522567439dfb9f3e72b21f9b25941a42f04a7cd",
		},
		{
			name:     "空数据",
			key:      "key",
			data:     "",
			expected: "5d5d139563c95b5967b9bd9a8c9b233a9dedb45072794cd232dc1b74832607d0",
		},
		{
			name:     "中文内容",
			key:      "secret",
			data:     "中文测试",
			expected: "5940ead740f400fd6dbc17d967881dabf15b235fc02cb39c2a77d192e0b92271",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hmacSHA256([]byte(tc.key), []byte(tc.data))
			hexResult := hex.EncodeToString(result)

			if tc.expected != "" && hexResult != tc.expected {
				t.Errorf("HMAC不匹配: got %s, want %s", hexResult, tc.expected)
			}

			// 验证长度（SHA256 = 32字节 = 64 hex字符）
			if len(hexResult) != 64 {
				t.Errorf("HMAC长度错误: got %d, want 64", len(hexResult))
			}
		})
	}
}

// TestDeriveSigningKey 测试签名密钥派生
func TestDeriveSigningKey(t *testing.T) {
	testCases := []struct {
		name    string
		secret  string
		dateStr string
		region  string
	}{
		{
			name:    "标准派生",
			secret:  "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
			dateStr: "20150830",
			region:  "us-east-1",
		},
		{
			name:    "不同区域",
			secret:  "test-secret",
			dateStr: "20231215",
			region:  "ap-northeast-1",
		},
		{
			name:    "自定义区域",
			secret:  "my-secret-key",
			dateStr: "20251207",
			region:  "custom-region",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := deriveSigningKey(tc.secret, tc.dateStr, tc.region)

			// 验证密钥长度
			if len(key) != 32 {
				t.Errorf("签名密钥长度错误: got %d, want 32", len(key))
			}

			// 验证同样输入产生同样输出
			key2 := deriveSigningKey(tc.secret, tc.dateStr, tc.region)
			if hex.EncodeToString(key) != hex.EncodeToString(key2) {
				t.Error("相同输入应该产生相同的签名密钥")
			}

			// 验证不同输入产生不同输出
			key3 := deriveSigningKey(tc.secret, tc.dateStr, "different-region")
			if hex.EncodeToString(key) == hex.EncodeToString(key3) {
				t.Error("不同区域应该产生不同的签名密钥")
			}
		})
	}
}

// TestGetCanonicalURI 测试规范URI
func TestGetCanonicalURI(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "空路径",
			path:     "",
			expected: "/",
		},
		{
			name:     "根路径",
			path:     "/",
			expected: "/",
		},
		{
			name:     "简单路径",
			path:     "/bucket/object",
			expected: "/bucket/object",
		},
		{
			name:     "带空格的路径",
			path:     "/bucket/my object.txt",
			expected: "/bucket/my%20object.txt",
		},
		{
			name:     "中文路径",
			path:     "/bucket/中文文件.txt",
			expected: "/bucket/%E4%B8%AD%E6%96%87%E6%96%87%E4%BB%B6.txt",
		},
		{
			name:     "特殊字符",
			path:     "/bucket/file+name.txt",
			expected: "/bucket/file+name.txt",
		},
		{
			name:     "多级路径",
			path:     "/bucket/dir1/dir2/file.txt",
			expected: "/bucket/dir1/dir2/file.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getCanonicalURI(tc.path)
			if result != tc.expected {
				t.Errorf("规范URI不匹配: got %q, want %q", result, tc.expected)
			}
		})
	}
}

// TestGetCanonicalQueryString 测试规范查询字符串
func TestGetCanonicalQueryString(t *testing.T) {
	testCases := []struct {
		name     string
		query    url.Values
		expected string
	}{
		{
			name:     "空查询",
			query:    url.Values{},
			expected: "",
		},
		{
			name: "单个参数",
			query: url.Values{
				"key": []string{"value"},
			},
			expected: "key=value",
		},
		{
			name: "多个参数按字母排序",
			query: url.Values{
				"z":    []string{"last"},
				"a":    []string{"first"},
				"m":    []string{"middle"},
			},
			expected: "a=first&m=middle&z=last",
		},
		{
			name: "同一键多个值",
			query: url.Values{
				"key": []string{"b", "a", "c"},
			},
			expected: "key=a&key=b&key=c",
		},
		{
			name: "需要URL编码的值",
			query: url.Values{
				"key": []string{"hello world"},
			},
			expected: "key=hello+world",
		},
		{
			name: "移除X-Amz-Signature",
			query: url.Values{
				"key":             []string{"value"},
				"X-Amz-Signature": []string{"should-be-removed"},
			},
			expected: "key=value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getCanonicalQueryString(tc.query)
			if result != tc.expected {
				t.Errorf("规范查询字符串不匹配: got %q, want %q", result, tc.expected)
			}
		})
	}
}

// TestCreateStringToSign 测试待签名字符串创建
func TestCreateStringToSign(t *testing.T) {
	dateTime := "20150830T123600Z"
	scope := "20150830/us-east-1/s3/aws4_request"
	canonicalRequest := "GET\n/\n\nhost:example.amazonaws.com\n\nhost\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	result := createStringToSign(dateTime, scope, canonicalRequest)

	// 验证格式
	lines := strings.Split(result, "\n")
	if len(lines) != 4 {
		t.Errorf("待签名字符串应该有4行: got %d", len(lines))
	}

	if lines[0] != algorithm {
		t.Errorf("第一行应该是算法: got %s, want %s", lines[0], algorithm)
	}

	if lines[1] != dateTime {
		t.Errorf("第二行应该是日期时间: got %s, want %s", lines[1], dateTime)
	}

	if lines[2] != scope {
		t.Errorf("第三行应该是scope: got %s, want %s", lines[2], scope)
	}

	// 第四行是规范请求的哈希
	if len(lines[3]) != 64 {
		t.Errorf("第四行应该是64字符的哈希: got %d", len(lines[3]))
	}
}

// TestAuthHeaderRegex 测试Authorization头解析
func TestAuthHeaderRegex(t *testing.T) {
	testCases := []struct {
		name          string
		header        string
		expectMatch   bool
		expectAccess  string
		expectDate    string
		expectRegion  string
		expectHeaders string
		expectSig     string
	}{
		{
			name:          "有效的Authorization头",
			header:        "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			expectMatch:   true,
			expectAccess:  "AKIAIOSFODNN7EXAMPLE",
			expectDate:    "20130524",
			expectRegion:  "us-east-1",
			expectHeaders: "host;x-amz-content-sha256;x-amz-date",
			expectSig:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			name:        "无效格式 - 缺少算法",
			header:      "Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abcd",
			expectMatch: false,
		},
		{
			name:        "无效格式 - 错误的签名长度",
			header:      "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=short",
			expectMatch: false,
		},
		{
			name:        "空头部",
			header:      "",
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := authHeaderRegex.FindStringSubmatch(tc.header)
			if tc.expectMatch {
				if matches == nil {
					t.Error("期望匹配但未匹配")
					return
				}
				if matches[1] != tc.expectAccess {
					t.Errorf("Access Key不匹配: got %s, want %s", matches[1], tc.expectAccess)
				}
				if matches[2] != tc.expectDate {
					t.Errorf("日期不匹配: got %s, want %s", matches[2], tc.expectDate)
				}
				if matches[3] != tc.expectRegion {
					t.Errorf("区域不匹配: got %s, want %s", matches[3], tc.expectRegion)
				}
				if matches[4] != tc.expectHeaders {
					t.Errorf("SignedHeaders不匹配: got %s, want %s", matches[4], tc.expectHeaders)
				}
				if matches[5] != tc.expectSig {
					t.Errorf("签名不匹配: got %s, want %s", matches[5], tc.expectSig)
				}
			} else {
				if matches != nil {
					t.Error("期望不匹配但匹配了")
				}
			}
		})
	}
}

// TestGetSecretKey 测试获取Secret Key
func TestGetSecretKey(t *testing.T) {
	// 设置测试配置
	setupTestConfig()

	t.Run("从全局配置获取", func(t *testing.T) {
		secret := getSecretKey("test-access-key")
		if secret != "test-secret-key" {
			t.Errorf("从配置获取Secret Key失败: got %s, want test-secret-key", secret)
		}
	})

	t.Run("不存在的Key", func(t *testing.T) {
		secret := getSecretKey("nonexistent-key")
		if secret != "" {
			t.Errorf("不存在的Key应该返回空: got %s", secret)
		}
	})
}

// TestCheckBucketPermission 测试桶权限检查
func TestCheckBucketPermission(t *testing.T) {
	setupTestConfig()

	t.Run("管理员Key有全部权限", func(t *testing.T) {
		// 读权限
		if !CheckBucketPermission("test-access-key", "any-bucket", false) {
			t.Error("管理员Key应该有读权限")
		}
		// 写权限
		if !CheckBucketPermission("test-access-key", "any-bucket", true) {
			t.Error("管理员Key应该有写权限")
		}
	})

	t.Run("无效Key无权限", func(t *testing.T) {
		if CheckBucketPermission("invalid-key", "any-bucket", false) {
			t.Error("无效Key不应该有权限")
		}
	})
}

// TestInitAPIKeyCache 测试API Key缓存初始化
func TestInitAPIKeyCache(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 重置全局缓存
	apiKeyCache = nil

	// 初始化缓存
	InitAPIKeyCache(store)

	if apiKeyCache == nil {
		t.Error("API Key缓存应该被初始化")
	}
}

// TestReloadAPIKeyCache 测试重载API Key缓存
func TestReloadAPIKeyCache(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	t.Run("缓存未初始化时重载", func(t *testing.T) {
		apiKeyCache = nil
		err := ReloadAPIKeyCache()
		if err != nil {
			t.Errorf("缓存未初始化时重载不应该返回错误: %v", err)
		}
	})

	t.Run("缓存已初始化时重载", func(t *testing.T) {
		InitAPIKeyCache(store)
		err := ReloadAPIKeyCache()
		if err != nil {
			t.Errorf("重载缓存失败: %v", err)
		}
	})
}

// TestVerifyRequest 测试请求验证
func TestVerifyRequest(t *testing.T) {
	setupTestConfig()

	t.Run("无Authorization头", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/bucket/object", nil)
		if VerifyRequest(req) {
			t.Error("无Authorization头应该验证失败")
		}
	})

	t.Run("无效的Authorization头格式", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/bucket/object", nil)
		req.Header.Set("Authorization", "Invalid-Format")
		if VerifyRequest(req) {
			t.Error("无效格式应该验证失败")
		}
	})

	t.Run("无效的Access Key", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/bucket/object", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=INVALID/20231215/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		req.Header.Set("X-Amz-Date", "20231215T120000Z")
		if VerifyRequest(req) {
			t.Error("无效Access Key应该验证失败")
		}
	})
}

// TestVerifyRequestAndGetAccessKey 测试请求验证并获取Access Key
func TestVerifyRequestAndGetAccessKey(t *testing.T) {
	setupTestConfig()

	t.Run("无Authorization头", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/bucket/object", nil)
		accessKey, ok := VerifyRequestAndGetAccessKey(req)
		if ok {
			t.Error("无Authorization头应该验证失败")
		}
		if accessKey != "" {
			t.Error("验证失败时Access Key应该为空")
		}
	})
}

// TestCreateCanonicalRequest 测试创建规范请求
func TestCreateCanonicalRequest(t *testing.T) {
	testCases := []struct {
		name          string
		method        string
		path          string
		query         string
		headers       map[string]string
		signedHeaders string
	}{
		{
			name:   "简单GET请求",
			method: "GET",
			path:   "/bucket/object",
			query:  "",
			headers: map[string]string{
				"Host":                 "s3.example.com",
				"X-Amz-Date":           "20231215T120000Z",
				"X-Amz-Content-Sha256": unsignedPayload,
			},
			signedHeaders: "host;x-amz-content-sha256;x-amz-date",
		},
		{
			name:   "PUT请求",
			method: "PUT",
			path:   "/bucket/object",
			query:  "",
			headers: map[string]string{
				"Host":                 "s3.example.com",
				"X-Amz-Date":           "20231215T120000Z",
				"X-Amz-Content-Sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				"Content-Type":         "application/octet-stream",
			},
			signedHeaders: "content-type;host;x-amz-content-sha256;x-amz-date",
		},
		{
			name:   "带查询参数",
			method: "GET",
			path:   "/bucket",
			query:  "list-type=2&prefix=test",
			headers: map[string]string{
				"Host":       "s3.example.com",
				"X-Amz-Date": "20231215T120000Z",
			},
			signedHeaders: "host;x-amz-date",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			urlStr := tc.path
			if tc.query != "" {
				urlStr += "?" + tc.query
			}
			req := httptest.NewRequest(tc.method, urlStr, nil)
			req.Host = tc.headers["Host"]
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			result := createCanonicalRequest(req, tc.signedHeaders)

			// 验证格式（6行）
			lines := strings.Split(result, "\n")
			if len(lines) < 6 {
				t.Errorf("规范请求应该至少有6部分: got %d", len(lines))
				return
			}

			// 验证HTTP方法
			if lines[0] != tc.method {
				t.Errorf("第一行应该是HTTP方法: got %s, want %s", lines[0], tc.method)
			}
		})
	}
}

// TestGetPayloadHash 测试Payload哈希计算
func TestGetPayloadHash(t *testing.T) {
	emptyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	t.Run("空body", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		hash := GetPayloadHash(req)
		if hash != emptyHash {
			t.Errorf("空body哈希不匹配: got %s, want %s", hash, emptyHash)
		}
	})

	t.Run("有内容的body", func(t *testing.T) {
		body := "test content"
		req := httptest.NewRequest("PUT", "/", strings.NewReader(body))
		hash := GetPayloadHash(req)

		// 计算预期哈希
		expected := sha256.Sum256([]byte(body))
		expectedHex := hex.EncodeToString(expected[:])

		if hash != expectedHex {
			t.Errorf("body哈希不匹配: got %s, want %s", hash, expectedHex)
		}
	})
}

// TestCalculateSignatureWithSecret 测试签名计算
func TestCalculateSignatureWithSecret(t *testing.T) {
	// 创建测试请求
	req := httptest.NewRequest("GET", "/bucket/object", nil)
	req.Host = "s3.example.com"
	req.Header.Set("X-Amz-Date", "20231215T120000Z")
	req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)

	dateStr := "20231215"
	region := "us-east-1"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	secretKey := "test-secret-key"

	sig1 := calculateSignatureWithSecret(req, dateStr, region, signedHeaders, secretKey)

	// 验证签名长度（64个hex字符）
	if len(sig1) != 64 {
		t.Errorf("签名长度错误: got %d, want 64", len(sig1))
	}

	// 验证相同输入产生相同签名
	sig2 := calculateSignatureWithSecret(req, dateStr, region, signedHeaders, secretKey)
	if sig1 != sig2 {
		t.Error("相同输入应该产生相同签名")
	}

	// 验证不同密钥产生不同签名
	sig3 := calculateSignatureWithSecret(req, dateStr, region, signedHeaders, "different-key")
	if sig1 == sig3 {
		t.Error("不同密钥应该产生不同签名")
	}
}

// TestVerifyPresignedURL 测试预签名URL验证
func TestVerifyPresignedURL(t *testing.T) {
	setupTestConfig()

	t.Run("缺少Credential参数", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/bucket/object?X-Amz-Signature=abc", nil)
		_, ok := verifyPresignedURL(req)
		if ok {
			t.Error("缺少Credential应该验证失败")
		}
	})

	t.Run("无效的Credential格式", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/bucket/object?X-Amz-Signature=abc&X-Amz-Credential=invalid", nil)
		_, ok := verifyPresignedURL(req)
		if ok {
			t.Error("无效Credential格式应该验证失败")
		}
	})

	t.Run("无效的Access Key", func(t *testing.T) {
		cred := "invalid-key/20231215/us-east-1/s3/aws4_request"
		req := httptest.NewRequest("GET", "/bucket/object?X-Amz-Signature=abc&X-Amz-Credential="+url.QueryEscape(cred), nil)
		_, ok := verifyPresignedURL(req)
		if ok {
			t.Error("无效Access Key应该验证失败")
		}
	})

	t.Run("缺少过期时间参数", func(t *testing.T) {
		cred := "test-access-key/20231215/us-east-1/s3/aws4_request"
		req := httptest.NewRequest("GET", "/bucket/object?X-Amz-Signature=abc&X-Amz-Credential="+url.QueryEscape(cred), nil)
		_, ok := verifyPresignedURL(req)
		if ok {
			t.Error("缺少过期时间应该验证失败")
		}
	})

	t.Run("已过期的URL", func(t *testing.T) {
		cred := "test-access-key/20231215/us-east-1/s3/aws4_request"
		// 使用过去的时间
		pastTime := time.Now().Add(-2 * time.Hour).Format("20060102T150405Z")
		urlStr := fmt.Sprintf("/bucket/object?X-Amz-Signature=abc&X-Amz-Credential=%s&X-Amz-Date=%s&X-Amz-Expires=3600",
			url.QueryEscape(cred), pastTime)
		req := httptest.NewRequest("GET", urlStr, nil)
		_, ok := verifyPresignedURL(req)
		if ok {
			t.Error("已过期URL应该验证失败")
		}
	})
}

// TestSignatureIntegration 测试签名验证完整流程
func TestSignatureIntegration(t *testing.T) {
	setupTestConfig()

	// 创建一个有效签名的请求
	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	region := "us-east-1"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	req := httptest.NewRequest("GET", "/test-bucket/test-object", nil)
	req.Host = "localhost"
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)

	// 计算签名
	signature := calculateSignatureWithSecret(req, dateStr, region, signedHeaders, config.Global.Auth.SecretAccessKey)

	// 构造Authorization头
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
		config.Global.Auth.AccessKeyID, dateStr, region, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	// 验证请求
	accessKey, ok := VerifyRequestAndGetAccessKey(req)
	if !ok {
		t.Error("有效签名的请求应该验证成功")
	}
	if accessKey != config.Global.Auth.AccessKeyID {
		t.Errorf("Access Key不匹配: got %s, want %s", accessKey, config.Global.Auth.AccessKeyID)
	}
}

// TestSignatureWithDifferentMethods 测试不同HTTP方法的签名
func TestSignatureWithDifferentMethods(t *testing.T) {
	setupTestConfig()

	methods := []string{"GET", "PUT", "POST", "DELETE", "HEAD"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			now := time.Now().UTC()
			dateStr := now.Format("20060102")
			amzDate := now.Format("20060102T150405Z")
			region := "us-east-1"
			signedHeaders := "host;x-amz-content-sha256;x-amz-date"

			req := httptest.NewRequest(method, "/test-bucket/test-object", nil)
			req.Host = "localhost"
			req.Header.Set("X-Amz-Date", amzDate)
			req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)

			// 计算签名
			signature := calculateSignatureWithSecret(req, dateStr, region, signedHeaders, config.Global.Auth.SecretAccessKey)

			// 构造Authorization头
			authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
				config.Global.Auth.AccessKeyID, dateStr, region, signedHeaders, signature)
			req.Header.Set("Authorization", authHeader)

			// 验证请求
			if !VerifyRequest(req) {
				t.Errorf("%s请求签名验证失败", method)
			}
		})
	}
}

// TestSignatureTampering 测试签名篡改检测
func TestSignatureTampering(t *testing.T) {
	setupTestConfig()

	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	region := "us-east-1"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	req := httptest.NewRequest("GET", "/test-bucket/test-object", nil)
	req.Host = "localhost"
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)

	// 计算正确签名
	signature := calculateSignatureWithSecret(req, dateStr, region, signedHeaders, config.Global.Auth.SecretAccessKey)

	t.Run("篡改签名", func(t *testing.T) {
		// 修改签名的一个字符
		tamperedSig := "0" + signature[1:]
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
			config.Global.Auth.AccessKeyID, dateStr, region, signedHeaders, tamperedSig)

		tamperedReq := httptest.NewRequest("GET", "/test-bucket/test-object", nil)
		tamperedReq.Host = "localhost"
		tamperedReq.Header.Set("X-Amz-Date", amzDate)
		tamperedReq.Header.Set("X-Amz-Content-Sha256", unsignedPayload)
		tamperedReq.Header.Set("Authorization", authHeader)

		if VerifyRequest(tamperedReq) {
			t.Error("篡改的签名应该验证失败")
		}
	})

	t.Run("篡改路径", func(t *testing.T) {
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
			config.Global.Auth.AccessKeyID, dateStr, region, signedHeaders, signature)

		// 使用不同的路径
		tamperedReq := httptest.NewRequest("GET", "/different-bucket/test-object", nil)
		tamperedReq.Host = "localhost"
		tamperedReq.Header.Set("X-Amz-Date", amzDate)
		tamperedReq.Header.Set("X-Amz-Content-Sha256", unsignedPayload)
		tamperedReq.Header.Set("Authorization", authHeader)

		if VerifyRequest(tamperedReq) {
			t.Error("篡改路径后签名应该验证失败")
		}
	})
}

// BenchmarkHmacSHA256 HMAC-SHA256性能测试
func BenchmarkHmacSHA256(b *testing.B) {
	key := []byte("test-secret-key")
	data := []byte("test data to hash")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hmacSHA256(key, data)
	}
}

// BenchmarkDeriveSigningKey 签名密钥派生性能测试
func BenchmarkDeriveSigningKey(b *testing.B) {
	secret := "test-secret-key"
	dateStr := "20231215"
	region := "us-east-1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		deriveSigningKey(secret, dateStr, region)
	}
}

// BenchmarkCalculateSignature 签名计算性能测试
func BenchmarkCalculateSignature(b *testing.B) {
	setupTestConfig()

	req := httptest.NewRequest("GET", "/bucket/object", nil)
	req.Host = "s3.example.com"
	req.Header.Set("X-Amz-Date", "20231215T120000Z")
	req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)

	dateStr := "20231215"
	region := "us-east-1"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateSignatureWithSecret(req, dateStr, region, signedHeaders, config.Global.Auth.SecretAccessKey)
	}
}

// BenchmarkVerifyRequest 请求验证性能测试
func BenchmarkVerifyRequest(b *testing.B) {
	setupTestConfig()

	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	region := "us-east-1"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	req := httptest.NewRequest("GET", "/test-bucket/test-object", nil)
	req.Host = "localhost"
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)

	signature := calculateSignatureWithSecret(req, dateStr, region, signedHeaders, config.Global.Auth.SecretAccessKey)
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
		config.Global.Auth.AccessKeyID, dateStr, region, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifyRequest(req)
	}
}
