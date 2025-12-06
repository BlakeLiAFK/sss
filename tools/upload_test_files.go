package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	accessKey  = "admin"
	secretKey  = "admin"
	region     = "us-east-1"
	baseURL    = "http://localhost:8080"
)

func main() {
	bucket := "my-test-bucket"
	if len(os.Args) > 1 {
		bucket = os.Args[1]
	}
	count := 120
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &count)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Printf("开始上传 %d 个测试文件到 %s 桶...\n", count, bucket)

	success := 0
	for i := 1; i <= count; i++ {
		key := fmt.Sprintf("test_file_%03d.txt", i)
		content := fmt.Sprintf("Test file %d - Created at %s\nLine 1\nLine 2\n", i, time.Now().Format(time.RFC3339))

		// 生成预签名URL
		presignedURL := generatePresignedURL("PUT", bucket, key, 5*time.Minute)

		// 创建请求
		req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader([]byte(content)))
		if err != nil {
			fmt.Printf("创建请求失败 %s: %v\n", key, err)
			continue
		}
		req.Header.Set("Content-Type", "text/plain")

		// 上传文件
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("上传失败 %s: %v\n", key, err)
			continue
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("上传失败 %s: %d %s\n", key, resp.StatusCode, string(body))
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		success++

		if i%20 == 0 {
			fmt.Printf("已上传 %d/%d 个文件\n", i, count)
		}
	}

	fmt.Printf("完成！成功上传 %d/%d 个测试文件到 %s 桶\n", success, count, bucket)
}

// generatePresignedURL 生成预签名URL（AWS Signature V4）
func generatePresignedURL(method, bucket, key string, expires time.Duration) string {
	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	expiresSeconds := int(expires.Seconds())

	// 构建URL
	u, _ := url.Parse(baseURL)
	u.Path = "/" + bucket + "/" + key

	// 查询参数
	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", accessKey+"/"+dateStr+"/"+region+"/s3/aws4_request")
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", fmt.Sprintf("%d", expiresSeconds))
	q.Set("X-Amz-SignedHeaders", "host")

	// 规范查询字符串（按字母排序）
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var canonicalQuery strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonicalQuery.WriteString("&")
		}
		canonicalQuery.WriteString(url.QueryEscape(k))
		canonicalQuery.WriteString("=")
		canonicalQuery.WriteString(url.QueryEscape(q.Get(k)))
	}

	// 规范请求
	canonicalRequest := strings.Join([]string{
		method,
		u.Path,
		canonicalQuery.String(),
		"host:" + u.Host + "\n",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")

	// 待签名字符串
	scope := dateStr + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// 计算签名密钥
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStr)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	q.Set("X-Amz-Signature", signature)
	u.RawQuery = q.Encode()

	return u.String()
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
