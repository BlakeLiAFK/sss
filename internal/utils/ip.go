package utils

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP 获取客户端真实 IP
// 支持各种 CDN/代理的头部：
// - CF-Connecting-IP (Cloudflare)
// - X-Real-IP (Nginx)
// - X-Forwarded-For (标准代理头)
// - True-Client-IP (Akamai, Cloudflare Enterprise)
// - X-Client-IP
// - Fastly-Client-IP (Fastly)
// - X-Cluster-Client-IP (Rackspace)
func GetClientIP(r *http.Request) string {
	// 按优先级检查各种头部
	headers := []string{
		"CF-Connecting-IP",    // Cloudflare
		"True-Client-IP",      // Akamai, Cloudflare Enterprise
		"X-Real-IP",           // Nginx
		"X-Client-IP",         // 通用
		"Fastly-Client-IP",    // Fastly CDN
		"X-Cluster-Client-IP", // Rackspace
	}

	for _, header := range headers {
		if ip := r.Header.Get(header); ip != "" {
			// 验证是否为有效 IP
			if parsedIP := net.ParseIP(strings.TrimSpace(ip)); parsedIP != nil {
				return parsedIP.String()
			}
		}
	}

	// 检查 X-Forwarded-For（可能包含多个 IP，取第一个）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if parsedIP := net.ParseIP(ip); parsedIP != nil {
				return parsedIP.String()
			}
		}
	}

	// 从 RemoteAddr 获取 IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr 可能不包含端口
		ip = r.RemoteAddr
	}

	// 处理 IPv6 本地地址
	if ip == "::1" {
		return "127.0.0.1"
	}

	return ip
}

// GetUserAgent 获取 User-Agent
func GetUserAgent(r *http.Request) string {
	ua := r.Header.Get("User-Agent")
	// 限制长度，防止存储过长的 UA
	if len(ua) > 500 {
		ua = ua[:500]
	}
	return ua
}

// IsPrivateIP 判断是否为内网 IP
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 检查是否为私有地址
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}
