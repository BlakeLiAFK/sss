package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"sss/internal/auth"
	"sss/internal/config"
	"sss/internal/storage"
)

// 测试用的凭证
const (
	testAccessKey = "TESTACCESSKEY123456"
	testSecretKey = "TESTSECRETKEY1234567890ABCDEFGHIJKLMNO"
	testRegion    = "us-east-1"
	testBucket    = "test-bucket"
)

// setupS3AuthTest 设置S3认证测试环境
func setupS3AuthTest(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "sss-s3auth-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	dbPath := tmpDir + "/metadata.db"
	dataPath := tmpDir + "/data"

	// 创建存储
	metadata, err := storage.NewMetadataStore(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("创建元数据存储失败: %v", err)
	}

	filestore, err := storage.NewFileStore(dataPath)
	if err != nil {
		metadata.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 初始化全局配置
	config.Global = &config.Config{
		Auth: config.AuthConfig{
			AccessKeyID:     testAccessKey,
			SecretAccessKey: testSecretKey,
		},
		Server: config.ServerConfig{
			Host:   "localhost",
			Port:   8080,
			Region: testRegion,
		},
	}

	// 初始化API Key缓存
	auth.InitAPIKeyCache(metadata)

	// 创建服务器
	server := NewServer(metadata, filestore)

	cleanup := func() {
		metadata.Close()
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

// signRequest 使用AWS Signature V4签名请求
func signRequest(req *http.Request, accessKey, secretKey, region string, payload []byte) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")

	// 设置必要的头部
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.Host)

	// 计算payload哈希
	var payloadHash string
	if payload != nil {
		hash := sha256.Sum256(payload)
		payloadHash = hex.EncodeToString(hash[:])
	} else {
		payloadHash = "UNSIGNED-PAYLOAD"
	}
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// 确定签名头部
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	if req.Header.Get("Content-Type") != "" {
		signedHeaders = "content-type;" + signedHeaders
	}

	// 创建规范请求
	canonicalRequest := createCanonicalRequestForTest(req, signedHeaders, payloadHash)

	// 创建待签名字符串
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, region)
	stringToSign := createStringToSignForTest(amzDate, scope, canonicalRequest)

	// 计算签名
	signingKey := deriveSigningKeyForTest(secretKey, dateStr, region)
	signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, []byte(stringToSign)))

	// 设置Authorization头
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
		accessKey, dateStr, region, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

// createCanonicalRequestForTest 创建规范请求
func createCanonicalRequestForTest(req *http.Request, signedHeaders, payloadHash string) string {
	// URI编码
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// 规范查询字符串
	canonicalQuery := getCanonicalQueryStringForTest(req.URL.Query())

	// 规范头部
	headerList := strings.Split(signedHeaders, ";")
	sort.Strings(headerList)
	var canonicalHeaders strings.Builder
	for _, h := range headerList {
		h = strings.ToLower(h)
		var value string
		if h == "host" {
			value = req.Host
		} else {
			value = req.Header.Get(h)
		}
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", h, strings.TrimSpace(value)))
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	)
}

// getCanonicalQueryStringForTest 获取规范查询字符串
func getCanonicalQueryStringForTest(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	var keys []string
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			pairs = append(pairs, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)))
		}
	}

	return strings.Join(pairs, "&")
}

// createStringToSignForTest 创建待签名字符串
func createStringToSignForTest(dateTime, scope, canonicalRequest string) string {
	hash := sha256.Sum256([]byte(canonicalRequest))
	return fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		dateTime,
		scope,
		hex.EncodeToString(hash[:]),
	)
}

// deriveSigningKeyForTest 派生签名密钥
func deriveSigningKeyForTest(secret, dateStr, region string) []byte {
	kDate := hmacSHA256ForTest([]byte("AWS4"+secret), []byte(dateStr))
	kRegion := hmacSHA256ForTest(kDate, []byte(region))
	kService := hmacSHA256ForTest(kRegion, []byte("s3"))
	kSigning := hmacSHA256ForTest(kService, []byte("aws4_request"))
	return kSigning
}

// hmacSHA256ForTest 计算HMAC-SHA256
func hmacSHA256ForTest(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// TestDirectPutObject 测试直接PUT操作（使用Authorization头，非预签名）
func TestDirectPutObject(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 首先创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)

	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)

	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d, body: %s", w.Code, w.Body.String())
	}

	// 2. 使用直接签名的PUT请求上传对象
	objectKey := "test-object.txt"
	objectContent := []byte("Hello, World! This is a test object.")

	putReq := httptest.NewRequest("PUT", "/"+testBucket+"/"+objectKey, bytes.NewReader(objectContent))
	putReq.Host = "localhost:8080"
	putReq.Header.Set("Content-Type", "text/plain")
	putReq.ContentLength = int64(len(objectContent))
	signRequest(putReq, testAccessKey, testSecretKey, testRegion, objectContent)

	w = httptest.NewRecorder()
	server.ServeHTTP(w, putReq)

	t.Logf("PUT响应状态码: %d", w.Code)
	t.Logf("PUT响应头: %v", w.Header())
	t.Logf("PUT响应体: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("直接PUT操作失败: 期望状态码 200, 实际 %d", w.Code)
		t.Errorf("响应体: %s", w.Body.String())
	}

	// 验证ETag头
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("PUT响应应包含ETag头")
	}
}

// TestDirectGetObject 测试直接GET操作
func TestDirectGetObject(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 2. 上传对象
	objectKey := "test-get.txt"
	objectContent := []byte("Test content for GET operation")
	putReq := httptest.NewRequest("PUT", "/"+testBucket+"/"+objectKey, bytes.NewReader(objectContent))
	putReq.Host = "localhost:8080"
	putReq.Header.Set("Content-Type", "text/plain")
	putReq.ContentLength = int64(len(objectContent))
	signRequest(putReq, testAccessKey, testSecretKey, testRegion, objectContent)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, putReq)
	if w.Code != http.StatusOK {
		t.Fatalf("上传对象失败: %d, %s", w.Code, w.Body.String())
	}

	// 3. GET对象
	getReq := httptest.NewRequest("GET", "/"+testBucket+"/"+objectKey, nil)
	getReq.Host = "localhost:8080"
	signRequest(getReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, getReq)

	t.Logf("GET响应状态码: %d", w.Code)
	t.Logf("GET响应内容: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("GET操作失败: 期望状态码 200, 实际 %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), objectContent) {
		t.Errorf("GET内容不匹配: 期望 %s, 实际 %s", objectContent, w.Body.String())
	}
}

// TestDirectDeleteObject 测试直接DELETE操作
func TestDirectDeleteObject(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 2. 上传对象
	objectKey := "test-delete.txt"
	objectContent := []byte("Test content for DELETE")
	putReq := httptest.NewRequest("PUT", "/"+testBucket+"/"+objectKey, bytes.NewReader(objectContent))
	putReq.Host = "localhost:8080"
	signRequest(putReq, testAccessKey, testSecretKey, testRegion, objectContent)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, putReq)
	if w.Code != http.StatusOK {
		t.Fatalf("上传对象失败: %d", w.Code)
	}

	// 3. DELETE对象
	deleteReq := httptest.NewRequest("DELETE", "/"+testBucket+"/"+objectKey, nil)
	deleteReq.Host = "localhost:8080"
	signRequest(deleteReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, deleteReq)

	t.Logf("DELETE响应状态码: %d", w.Code)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE操作失败: 期望状态码 204 或 200, 实际 %d", w.Code)
	}

	// 4. 验证对象已删除
	headReq := httptest.NewRequest("HEAD", "/"+testBucket+"/"+objectKey, nil)
	headReq.Host = "localhost:8080"
	signRequest(headReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, headReq)

	if w.Code != http.StatusNotFound {
		t.Errorf("对象应该已被删除, 期望 404, 实际 %d", w.Code)
	}
}

// TestListBuckets 测试ListBuckets操作
func TestListBuckets(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 创建几个bucket
	buckets := []string{"bucket-a", "bucket-b", "bucket-c"}
	for _, bucket := range buckets {
		req := httptest.NewRequest("PUT", "/"+bucket, nil)
		req.Host = "localhost:8080"
		signRequest(req, testAccessKey, testSecretKey, testRegion, nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("创建Bucket %s 失败: %d", bucket, w.Code)
		}
	}

	// 2. ListBuckets
	listReq := httptest.NewRequest("GET", "/", nil)
	listReq.Host = "localhost:8080"
	signRequest(listReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, listReq)

	t.Logf("ListBuckets响应状态码: %d", w.Code)
	t.Logf("ListBuckets响应体: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("ListBuckets操作失败: 期望状态码 200, 实际 %d", w.Code)
	}

	// 验证响应包含创建的bucket
	body := w.Body.String()
	for _, bucket := range buckets {
		if !strings.Contains(body, bucket) {
			t.Errorf("ListBuckets响应应该包含bucket: %s", bucket)
		}
	}
}

// TestListObjects 测试ListObjects操作
func TestListObjects(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 2. 上传多个对象
	objects := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
	for _, obj := range objects {
		content := []byte("Content of " + obj)
		putReq := httptest.NewRequest("PUT", "/"+testBucket+"/"+obj, bytes.NewReader(content))
		putReq.Host = "localhost:8080"
		signRequest(putReq, testAccessKey, testSecretKey, testRegion, content)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, putReq)
		if w.Code != http.StatusOK {
			t.Fatalf("上传对象 %s 失败: %d", obj, w.Code)
		}
	}

	// 3. ListObjects
	listReq := httptest.NewRequest("GET", "/"+testBucket, nil)
	listReq.Host = "localhost:8080"
	signRequest(listReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, listReq)

	t.Logf("ListObjects响应状态码: %d", w.Code)
	t.Logf("ListObjects响应体: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("ListObjects操作失败: 期望状态码 200, 实际 %d", w.Code)
	}

	// 验证响应包含上传的对象
	body := w.Body.String()
	for _, obj := range objects {
		if !strings.Contains(body, obj) {
			t.Errorf("ListObjects响应应该包含对象: %s", obj)
		}
	}
}

// TestHeadObject 测试HeadObject操作
func TestHeadObject(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 2. 上传对象
	objectKey := "test-head.txt"
	objectContent := []byte("Test content for HEAD operation")
	putReq := httptest.NewRequest("PUT", "/"+testBucket+"/"+objectKey, bytes.NewReader(objectContent))
	putReq.Host = "localhost:8080"
	putReq.Header.Set("Content-Type", "text/plain")
	signRequest(putReq, testAccessKey, testSecretKey, testRegion, objectContent)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, putReq)
	if w.Code != http.StatusOK {
		t.Fatalf("上传对象失败: %d", w.Code)
	}

	// 3. HEAD对象
	headReq := httptest.NewRequest("HEAD", "/"+testBucket+"/"+objectKey, nil)
	headReq.Host = "localhost:8080"
	signRequest(headReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, headReq)

	t.Logf("HEAD响应状态码: %d", w.Code)
	t.Logf("HEAD响应头: %v", w.Header())

	if w.Code != http.StatusOK {
		t.Errorf("HEAD操作失败: 期望状态码 200, 实际 %d", w.Code)
	}

	// 验证头部
	if w.Header().Get("Content-Length") == "" {
		t.Error("HEAD响应应包含Content-Length头")
	}
	if w.Header().Get("ETag") == "" {
		t.Error("HEAD响应应包含ETag头")
	}
}

// TestAuthWithoutSignature 测试没有签名的请求被拒绝
func TestAuthWithoutSignature(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 尝试无签名请求
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "localhost:8080"
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// 无签名的根路径会被当做静态文件请求处理（因为User-Agent为空）
	// 让我们测试一个需要认证的API路径
	req2 := httptest.NewRequest("PUT", "/test-bucket", nil)
	req2.Host = "localhost:8080"
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("无签名请求应该被拒绝: 期望状态码 403, 实际 %d", w2.Code)
	}
}

// TestAuthWithWrongSignature 测试错误签名的请求被拒绝
func TestAuthWithWrongSignature(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "localhost:8080"
	// 使用错误的Secret Key签名
	signRequest(req, testAccessKey, "WRONG_SECRET_KEY_12345678901234567890", testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	t.Logf("错误签名响应状态码: %d", w.Code)
	t.Logf("错误签名响应体: %s", w.Body.String())

	if w.Code != http.StatusForbidden {
		t.Errorf("错误签名请求应该被拒绝: 期望状态码 403, 实际 %d", w.Code)
	}
}

// TestCopyObject 测试CopyObject操作
func TestCopyObject(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 2. 上传源对象
	sourceKey := "source-file.txt"
	objectContent := []byte("Source content for COPY operation")
	putReq := httptest.NewRequest("PUT", "/"+testBucket+"/"+sourceKey, bytes.NewReader(objectContent))
	putReq.Host = "localhost:8080"
	signRequest(putReq, testAccessKey, testSecretKey, testRegion, objectContent)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, putReq)
	if w.Code != http.StatusOK {
		t.Fatalf("上传源对象失败: %d", w.Code)
	}

	// 3. 复制对象
	destKey := "dest-file.txt"
	copyReq := httptest.NewRequest("PUT", "/"+testBucket+"/"+destKey, nil)
	copyReq.Host = "localhost:8080"
	copyReq.Header.Set("x-amz-copy-source", "/"+testBucket+"/"+sourceKey)
	signRequest(copyReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, copyReq)

	t.Logf("COPY响应状态码: %d", w.Code)
	t.Logf("COPY响应体: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("COPY操作失败: 期望状态码 200, 实际 %d", w.Code)
	}

	// 4. 验证复制的对象
	getReq := httptest.NewRequest("GET", "/"+testBucket+"/"+destKey, nil)
	getReq.Host = "localhost:8080"
	signRequest(getReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, getReq)

	if w.Code != http.StatusOK {
		t.Errorf("获取复制对象失败: %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), objectContent) {
		t.Errorf("复制对象内容不匹配")
	}
}

// TestAPIKeyPermission 测试API Key权限控制
func TestAPIKeyPermission(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 用管理员Key创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 使用不存在的Key应该被拒绝
	invalidReq := httptest.NewRequest("GET", "/"+testBucket, nil)
	invalidReq.Host = "localhost:8080"
	signRequest(invalidReq, "INVALID_ACCESS_KEY", "INVALID_SECRET_KEY_12345678901234567890", testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, invalidReq)

	if w.Code != http.StatusForbidden {
		t.Errorf("无效API Key应该被拒绝: 期望 403, 实际 %d", w.Code)
	}
}

// TestMultipartUpload 测试多段上传
func TestMultipartUpload(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 1. 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	objectKey := "multipart-test.bin"

	// 2. 初始化多段上传
	initReq := httptest.NewRequest("POST", "/"+testBucket+"/"+objectKey+"?uploads", nil)
	initReq.Host = "localhost:8080"
	signRequest(initReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, initReq)

	t.Logf("InitiateMultipartUpload响应状态码: %d", w.Code)
	t.Logf("InitiateMultipartUpload响应体: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("InitiateMultipartUpload失败: 期望 200, 实际 %d", w.Code)
	}

	// 从响应中提取UploadId
	body := w.Body.String()
	if !strings.Contains(body, "UploadId") {
		t.Fatalf("响应应包含UploadId")
	}
}

// TestSignatureDebug 签名调试测试 - 详细打印签名过程
func TestSignatureDebug(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 详细测试PUT签名
	objectKey := "debug-test.txt"
	objectContent := []byte("Debug test content")

	req := httptest.NewRequest("PUT", "/"+testBucket+"/"+objectKey, bytes.NewReader(objectContent))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "text/plain")
	req.ContentLength = int64(len(objectContent))

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")

	t.Logf("=== 签名调试信息 ===")
	t.Logf("Method: %s", req.Method)
	t.Logf("Path: %s", req.URL.Path)
	t.Logf("Host: %s", req.Host)
	t.Logf("X-Amz-Date: %s", amzDate)
	t.Logf("DateStr: %s", dateStr)
	t.Logf("Region: %s", testRegion)
	t.Logf("AccessKey: %s", testAccessKey)
	t.Logf("SecretKey: %s...", testSecretKey[:10])

	// 计算payload哈希
	hash := sha256.Sum256(objectContent)
	payloadHash := hex.EncodeToString(hash[:])
	t.Logf("Payload Hash: %s", payloadHash)

	signRequest(req, testAccessKey, testSecretKey, testRegion, objectContent)

	t.Logf("Authorization Header: %s", req.Header.Get("Authorization"))

	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	t.Logf("Response Code: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("PUT操作应该成功: 期望 200, 实际 %d", w.Code)
	}
}

// BenchmarkSignRequest 签名性能测试
func BenchmarkSignRequest(b *testing.B) {
	content := []byte("Benchmark test content")
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("PUT", "/bucket/key", bytes.NewReader(content))
		req.Host = "localhost:8080"
		signRequest(req, testAccessKey, testSecretKey, testRegion, content)
	}
}

// TestUnsignedPayload 测试UNSIGNED-PAYLOAD
func TestUnsignedPayload(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 使用UNSIGNED-PAYLOAD上传
	objectKey := "unsigned-payload.txt"
	objectContent := []byte("Content with unsigned payload")

	req := httptest.NewRequest("PUT", "/"+testBucket+"/"+objectKey, bytes.NewReader(objectContent))
	req.Host = "localhost:8080"
	req.ContentLength = int64(len(objectContent))

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.Host)
	req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD") // 使用UNSIGNED-PAYLOAD

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	// 创建规范请求（使用UNSIGNED-PAYLOAD）
	canonicalRequest := createCanonicalRequestForTest(req, signedHeaders, "UNSIGNED-PAYLOAD")
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, testRegion)
	stringToSign := createStringToSignForTest(amzDate, scope, canonicalRequest)
	signingKey := deriveSigningKeyForTest(testSecretKey, dateStr, testRegion)
	signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
		testAccessKey, dateStr, testRegion, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	t.Logf("UNSIGNED-PAYLOAD PUT响应状态码: %d", w.Code)
	t.Logf("UNSIGNED-PAYLOAD PUT响应体: %s", w.Body.String())

	if w.Code != http.StatusOK {
		t.Errorf("UNSIGNED-PAYLOAD PUT应该成功: 期望 200, 实际 %d", w.Code)
	}

	// 验证对象已上传
	getReq := httptest.NewRequest("GET", "/"+testBucket+"/"+objectKey, nil)
	getReq.Host = "localhost:8080"
	signRequest(getReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, getReq)

	if w.Code != http.StatusOK {
		t.Errorf("获取上传对象失败: %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), objectContent) {
		t.Errorf("对象内容不匹配")
	}
}

// TestStreamingUpload 测试流式上传（模拟大文件）
func TestStreamingUpload(t *testing.T) {
	server, cleanup := setupS3AuthTest(t)
	defer cleanup()

	// 创建bucket
	createBucketReq := httptest.NewRequest("PUT", "/"+testBucket, nil)
	createBucketReq.Host = "localhost:8080"
	signRequest(createBucketReq, testAccessKey, testSecretKey, testRegion, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, createBucketReq)
	if w.Code != http.StatusOK {
		t.Fatalf("创建Bucket失败: %d", w.Code)
	}

	// 模拟流式上传（使用UNSIGNED-PAYLOAD因为流无法预计算哈希）
	objectKey := "streaming-upload.bin"
	objectContent := make([]byte, 1024*1024) // 1MB
	for i := range objectContent {
		objectContent[i] = byte(i % 256)
	}

	req := httptest.NewRequest("PUT", "/"+testBucket+"/"+objectKey, bytes.NewReader(objectContent))
	req.Host = "localhost:8080"
	req.ContentLength = int64(len(objectContent))
	req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.Host)

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := createCanonicalRequestForTest(req, signedHeaders, "UNSIGNED-PAYLOAD")
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, testRegion)
	stringToSign := createStringToSignForTest(amzDate, scope, canonicalRequest)
	signingKey := deriveSigningKeyForTest(testSecretKey, dateStr, testRegion)
	signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
		testAccessKey, dateStr, testRegion, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	t.Logf("流式上传响应状态码: %d", w.Code)

	if w.Code != http.StatusOK {
		t.Errorf("流式上传应该成功: 期望 200, 实际 %d", w.Code)
	}

	// 验证上传成功
	headReq := httptest.NewRequest("HEAD", "/"+testBucket+"/"+objectKey, nil)
	headReq.Host = "localhost:8080"
	signRequest(headReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, headReq)

	if w.Code != http.StatusOK {
		t.Errorf("HEAD对象失败: %d", w.Code)
	}

	// 下载并验证内容
	getReq := httptest.NewRequest("GET", "/"+testBucket+"/"+objectKey, nil)
	getReq.Host = "localhost:8080"
	signRequest(getReq, testAccessKey, testSecretKey, testRegion, nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, getReq)

	if w.Code != http.StatusOK {
		t.Errorf("GET对象失败: %d", w.Code)
	}

	downloadedContent, _ := io.ReadAll(w.Body)
	if !bytes.Equal(downloadedContent, objectContent) {
		t.Errorf("下载内容与上传内容不匹配")
	}
}
