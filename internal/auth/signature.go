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
	"strconv"
	"strings"
	"time"

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
var authHeaderRegex = regexp.MustCompile(`^AWS4-HMAC-SHA256\s+Credential=([^/]+)/(\d{8})/([^/]+)/s3/aws4_request,\s*SignedHeaders=([^,]+),\s*Signature=([a-f0-9]{64})$`)

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

	amzDate := r.Header.Get("X-Amz-Date")
	dateHeader := "x-amz-date"
	if amzDate == "" {
		amzDate = r.Header.Get("Date")
		dateHeader = "date"
	}
	stamp, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil && dateHeader == "date" {
		stamp, err = http.ParseTime(amzDate)
	}
	if err != nil || stamp.Format("20060102") != dateStr || time.Since(stamp) > 15*time.Minute || time.Until(stamp) > 15*time.Minute {
		return "", false
	}
	if !strings.Contains(";"+signedHeaders+";", ";host;") || !strings.Contains(";"+signedHeaders+";", ";"+dateHeader+";") {
		return "", false
	}
	if !validSignedHeaders(r, signedHeaders, false) {
		return "", false
	}
	payload := r.Header.Get("X-Amz-Content-Sha256")
	if payload != "" && payload != unsignedPayload {
		if b, err := hex.DecodeString(payload); err != nil || len(b) != 32 {
			return "", false
		}
	}
	// 获取对应的 Secret Key
	secretKey := getSecretKey(accessKey)
	if secretKey == "" {
		utils.Debug("invalid access key", "got", accessKey)
		return "", false
	}

	// 计算签名
	calculatedSig := calculateSignatureWithSecret(r, dateStr, region, signedHeaders, secretKey)
	if !hmac.Equal([]byte(calculatedSig), []byte(signature)) {
		utils.Debug("signature mismatch", "calculated", calculatedSig, "provided", signature)
		return "", false
	}

	return accessKey, true
}

// getSecretKey 获取 Access Key 对应的 Secret Key
func getSecretKey(accessKeyID string) string {
	// 从缓存中获取
	if apiKeyCache != nil {
		if secret, ok := apiKeyCache.GetSecretKey(accessKeyID); ok {
			return secret
		}
	}
	return ""
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
		segments[i] = awsEscape(seg)
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
			pairs = append(pairs, fmt.Sprintf("%s=%s", awsEscape(k), awsEscape(v)))
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
	if len(parts) != 5 || parts[3] != "s3" || parts[4] != "aws4_request" || query.Get("X-Amz-Algorithm") != algorithm {
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

	expireSec, err := strconv.Atoi(expires)
	if err != nil || expireSec < 1 || expireSec > 604800 || time.Until(t) > 15*time.Minute || t.Format("20060102") != dateStr || time.Now().After(t.Add(time.Duration(expireSec)*time.Second)) {
		utils.Debug("presigned URL expired")
		return "", false
	}

	// 验证签名
	providedSig := query.Get("X-Amz-Signature")
	signedHeaders := query.Get("X-Amz-SignedHeaders")
	if !validSignedHeaders(r, signedHeaders, true) {
		return "", false
	}

	// 创建规范请求（不包含 X-Amz-Signature）
	queryWithoutSig := make(url.Values)
	for k, v := range query {
		if k != "X-Amz-Signature" {
			queryWithoutSig[k] = v
		}
	}

	canonicalURI := getCanonicalURI(r.URL.Path)
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

	if hmac.Equal([]byte(calculatedSig), []byte(providedSig)) {
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

func awsEscape(value string) string { return strings.ReplaceAll(url.QueryEscape(value), "+", "%20") }

// Require every Amazon operation header to be authenticated. The payload hash
// is already bound by the canonical payload line for header authentication.
func validSignedHeaders(r *http.Request, names string, presigned bool) bool {
	signed := make(map[string]bool)
	previous := ""
	for _, name := range strings.Split(names, ";") {
		if name == "" || name != strings.ToLower(name) || (previous != "" && name <= previous) {
			return false
		}
		if name != "host" && name != "content-length" && len(r.Header.Values(name)) == 0 {
			return false
		}
		signed[name] = true
		previous = name
	}
	if !signed["host"] {
		return false
	}
	for name := range r.Header {
		name = strings.ToLower(name)
		if !strings.HasPrefix(name, "x-amz-") || signed[name] {
			continue
		}
		if name == "x-amz-content-sha256" && (!presigned || r.Header.Get(name) == unsignedPayload) {
			continue
		}
		return false
	}
	return true
}
