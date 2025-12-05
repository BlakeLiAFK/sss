package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"sss/internal/config"
)

// GeneratePresignedURL 生成预签名 URL
func GeneratePresignedURL(method, bucket, key string, expires time.Duration) string {
	cfg := config.Global

	// 构建 URL
	host := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.Host == "0.0.0.0" {
		host = fmt.Sprintf("localhost:%d", cfg.Server.Port)
	}

	path := fmt.Sprintf("/%s/%s", bucket, key)

	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	credential := fmt.Sprintf("%s/%s/%s/s3/aws4_request",
		cfg.Auth.AccessKeyID, dateStr, cfg.Server.Region)

	// 构建查询参数
	params := url.Values{
		"X-Amz-Algorithm":     {algorithm},
		"X-Amz-Credential":    {credential},
		"X-Amz-Date":          {amzDate},
		"X-Amz-Expires":       {fmt.Sprintf("%d", int(expires.Seconds()))},
		"X-Amz-SignedHeaders": {"host"},
	}

	// 规范查询字符串
	canonicalQuery := getCanonicalQueryStringForPresign(params)

	// 规范请求
	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		path,
		canonicalQuery,
		canonicalHeaders,
		"host",
		unsignedPayload,
	)

	// 待签名字符串
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, cfg.Server.Region)
	hash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		scope,
		hex.EncodeToString(hash[:]),
	)

	// 计算签名
	signingKey := deriveSigningKey(cfg.Auth.SecretAccessKey, dateStr, cfg.Server.Region)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// 构建最终 URL
	return fmt.Sprintf("http://%s%s?%s&X-Amz-Signature=%s",
		host, path, canonicalQuery, signature)
}

func getCanonicalQueryStringForPresign(params url.Values) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		for _, v := range params[k] {
			pairs = append(pairs, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)))
		}
	}
	return strings.Join(pairs, "&")
}
