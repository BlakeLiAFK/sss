package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// 全局 API Key 缓存
var apiKeyCache *storage.APIKeyCache

// InitAPIKeyCache 初始化 API Key 缓存
func InitAPIKeyCache(store *storage.MetadataStore) {
	apiKeyCache = storage.NewAPIKeyCache(store)
}

// ReloadAPIKeyCache 重新加载 API Key 缓存
func ReloadAPIKeyCache() error {
	if apiKeyCache != nil {
		return apiKeyCache.Reload()
	}
	return nil
}

// CheckBucketPermission 检查 API Key 对桶的访问权限
func CheckBucketPermission(accessKeyID, bucket string, needWrite bool) bool {
	// 如果使用旧配置的管理员 Key，拥有全部权限
	if config.Global.Auth.AccessKeyID != "" &&
		accessKeyID == config.Global.Auth.AccessKeyID {
		return true
	}

	// 从缓存检查权限
	if apiKeyCache != nil {
		return apiKeyCache.CheckPermission(accessKeyID, bucket, needWrite)
	}
	return false
}

const (
	algorithm       = "AWS4-HMAC-SHA256"
	serviceName     = "s3"
	terminationStr  = "aws4_request"
	unsignedPayload = "UNSIGNED-PAYLOAD"
)

// 解析 Authorization 头
var authHeaderRegex = regexp.MustCompile(`AWS4-HMAC-SHA256\s+Credential=([^/]+)/(\d{8})/([^/]+)/s3/aws4_request,\s*SignedHeaders=([^,]+),\s*Signature=([a-f0-9]+)`)

// VerifyRequest 验证请求签名，返回是否验证成功
func VerifyRequest(r *http.Request) bool {
	_, ok := VerifyRequestAndGetAccessKey(r)
	return ok
}

// VerifyRequestAndGetAccessKey 验证请求签名并返回 Access Key ID
func VerifyRequestAndGetAccessKey(r *http.Request) (string, bool) {
	// 检查是否是预签名 URL
	if r.URL.Query().Get("X-Amz-Signature") != "" {
		return verifyPresignedURL(r)
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}

	matches := authHeaderRegex.FindStringSubmatch(authHeader)
	if matches == nil {
		utils.Debug("invalid auth header format", "header", authHeader)
		return "", false
	}

	accessKey := matches[1]
	dateStr := matches[2]
	region := matches[3]
	signedHeaders := matches[4]
	signature := matches[5]

	// 获取对应的 Secret Key
	secretKey := getSecretKey(accessKey)
	if secretKey == "" {
		utils.Debug("invalid access key", "got", accessKey)
		return "", false
	}

	// 计算签名
	calculatedSig := calculateSignatureWithSecret(r, dateStr, region, signedHeaders, secretKey)
	if calculatedSig != signature {
		utils.Debug("signature mismatch", "calculated", calculatedSig, "provided", signature)
		return "", false
	}

	return accessKey, true
}

// getSecretKey 获取 Access Key 对应的 Secret Key
func getSecretKey(accessKeyID string) string {
	// 先检查旧配置中的管理员 Key
	if config.Global.Auth.AccessKeyID != "" &&
		accessKeyID == config.Global.Auth.AccessKeyID {
		return config.Global.Auth.SecretAccessKey
	}

	// 从缓存中获取
	if apiKeyCache != nil {
		if secret, ok := apiKeyCache.GetSecretKey(accessKeyID); ok {
			return secret
		}
	}
	return ""
}

// calculateSignature 计算请求签名（使用配置中的密钥，兼容旧代码）
func calculateSignature(r *http.Request, dateStr, region, signedHeaders string) string {
	return calculateSignatureWithSecret(r, dateStr, region, signedHeaders, config.Global.Auth.SecretAccessKey)
}

// calculateSignatureWithSecret 使用指定密钥计算请求签名
func calculateSignatureWithSecret(r *http.Request, dateStr, region, signedHeaders, secretKey string) string {
	// 获取请求时间
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		amzDate = r.Header.Get("Date")
	}

	// 1. 创建规范请求
	canonicalRequest := createCanonicalRequest(r, signedHeaders)
	utils.Debug("canonical request", "request", canonicalRequest)

	// 2. 创建待签名字符串
	scope := fmt.Sprintf("%s/%s/%s/%s", dateStr, region, serviceName, terminationStr)
	stringToSign := createStringToSign(amzDate, scope, canonicalRequest)
	utils.Debug("string to sign", "string", stringToSign)

	// 3. 计算签名
	signingKey := deriveSigningKey(secretKey, dateStr, region)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	return signature
}

// createCanonicalRequest 创建规范请求
func createCanonicalRequest(r *http.Request, signedHeaders string) string {
	// HTTP 方法
	method := r.Method

	// 规范 URI
	canonicalURI := getCanonicalURI(r.URL.Path)

	// 规范查询字符串
	canonicalQuery := getCanonicalQueryString(r.URL.Query())

	// 规范头部
	headerList := strings.Split(signedHeaders, ";")
	var canonicalHeaders strings.Builder
	for _, h := range headerList {
		h = strings.ToLower(h)
		var value string
		if h == "host" {
			value = r.Host
		} else {
			value = r.Header.Get(h)
		}
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", h, strings.TrimSpace(value)))
	}

	// Payload 哈希
	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = unsignedPayload
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	)
}

// createStringToSign 创建待签名字符串
func createStringToSign(dateTime, scope, canonicalRequest string) string {
	hash := sha256.Sum256([]byte(canonicalRequest))
	return fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		dateTime,
		scope,
		hex.EncodeToString(hash[:]),
	)
}

// deriveSigningKey 派生签名密钥
func deriveSigningKey(secret, dateStr, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStr))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte(terminationStr))
	return kSigning
}

// hmacSHA256 计算 HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// getCanonicalURI 获取规范 URI
func getCanonicalURI(path string) string {
	if path == "" {
		return "/"
	}
	// URI 编码，但保留斜杠
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

// getCanonicalQueryString 获取规范查询字符串
func getCanonicalQueryString(query url.Values) string {
	// 移除签名相关参数
	delete(query, "X-Amz-Signature")

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

// verifyPresignedURL 验证预签名 URL，返回 access key ID
func verifyPresignedURL(r *http.Request) (string, bool) {
	query := r.URL.Query()

	// 解析参数
	credential := query.Get("X-Amz-Credential")
	if credential == "" {
		return "", false
	}
	// Credential 格式: accessKey/date/region/s3/aws4_request
	parts := strings.Split(credential, "/")
	if len(parts) != 5 {
		return "", false
	}

	accessKeyID := parts[0]
	dateStr := parts[1]
	region := parts[2]

	// 获取对应的 Secret Key
	secretKey := getSecretKey(accessKeyID)
	if secretKey == "" {
		utils.Debug("invalid access key in presigned URL", "got", accessKeyID)
		return "", false
	}

	// 检查过期时间
	amzDate := query.Get("X-Amz-Date")
	expires := query.Get("X-Amz-Expires")
	if amzDate == "" || expires == "" {
		return "", false
	}

	t, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return "", false
	}

	var expireSec int
	fmt.Sscanf(expires, "%d", &expireSec)
	if time.Now().After(t.Add(time.Duration(expireSec) * time.Second)) {
		utils.Debug("presigned URL expired")
		return "", false
	}

	// 验证签名
	providedSig := query.Get("X-Amz-Signature")
	signedHeaders := query.Get("X-Amz-SignedHeaders")
	if signedHeaders == "" {
		signedHeaders = "host"
	}

	// 创建规范请求（不包含 X-Amz-Signature）
	queryWithoutSig := make(url.Values)
	for k, v := range query {
		if k != "X-Amz-Signature" {
			queryWithoutSig[k] = v
		}
	}

	// 对URL路径进行解码，因为浏览器会自动编码中文字符
	// 但生成预签名URL时使用的是原始未编码的路径
	decodedPath, err := url.PathUnescape(r.URL.Path)
	if err != nil {
		utils.Debug("failed to decode path", "path", r.URL.Path, "error", err)
		decodedPath = r.URL.Path // 解码失败则使用原路径
	}
	canonicalURI := decodedPath
	canonicalQuery := getCanonicalQueryString(queryWithoutSig)

	headerList := strings.Split(signedHeaders, ";")
	var canonicalHeaders strings.Builder
	for _, h := range headerList {
		h = strings.ToLower(h)
		var value string
		if h == "host" {
			value = r.Host
		} else {
			value = r.Header.Get(h)
		}
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", h, strings.TrimSpace(value)))
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		r.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		unsignedPayload,
	)

	scope := fmt.Sprintf("%s/%s/%s/%s", dateStr, region, serviceName, terminationStr)
	stringToSign := createStringToSign(amzDate, scope, canonicalRequest)
	signingKey := deriveSigningKey(secretKey, dateStr, region)
	calculatedSig := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	if calculatedSig == providedSig {
		return accessKeyID, true
	}
	return "", false
}

// GetPayloadHash 计算请求体哈希
func GetPayloadHash(r *http.Request) string {
	if r.Body == nil {
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // empty string hash
	}

	body, _ := io.ReadAll(r.Body)
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}
