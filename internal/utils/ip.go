package utils

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

// Cloudflare IP 范围预设（截至 2024 年）
// 来源：https://www.cloudflare.com/ips/
var CloudflareIPRanges = []string{
	// IPv4
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	// IPv6
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

// TrustedProxyCache 信任代理缓存
type TrustedProxyCache struct {
	mu       sync.RWMutex
	cidrs    []*net.IPNet // 预解析的 CIDR
	rawValue string       // 原始配置值（用于比较是否需要更新）
}

// 全局信任代理缓存
var trustedProxyCache = &TrustedProxyCache{}

// ReloadTrustedProxies 重新加载信任代理配置
// proxyCIDRs: 逗号分隔的 IP/CIDR 列表，如 "173.245.48.0/20,103.21.244.0/22"
func ReloadTrustedProxies(proxyCIDRs string) {
	trustedProxyCache.mu.Lock()
	defer trustedProxyCache.mu.Unlock()

	// 如果配置没变，不需要更新
	if trustedProxyCache.rawValue == proxyCIDRs {
		return
	}

	trustedProxyCache.rawValue = proxyCIDRs
	trustedProxyCache.cidrs = nil

	if proxyCIDRs == "" {
		return
	}

	parts := strings.Split(proxyCIDRs, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 如果没有 /，假设是单个 IP，添加 /32 或 /128
		if !strings.Contains(part, "/") {
			ip := net.ParseIP(part)
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				part = part + "/32"
			} else {
				part = part + "/128"
			}
		}

		_, cidr, err := net.ParseCIDR(part)
		if err != nil {
			// 跳过无效的 CIDR
			continue
		}
		trustedProxyCache.cidrs = append(trustedProxyCache.cidrs, cidr)
	}
}

// IsTrustedProxy 检查 IP 是否是信任的代理
func IsTrustedProxy(ipStr string) bool {
	trustedProxyCache.mu.RLock()
	defer trustedProxyCache.mu.RUnlock()

	// 如果没有配置信任代理，不信任任何代理
	if len(trustedProxyCache.cidrs) == 0 {
		return false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, cidr := range trustedProxyCache.cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// GetDirectIP 获取直连 IP（RemoteAddr）
// 这是 TCP 连接的真实 IP，不受代理头影响
func GetDirectIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	// 处理 IPv6 本地地址
	if ip == "::1" {
		return "127.0.0.1"
	}

	return ip
}

// GetForwardedIP 获取代理头中的客户端 IP
// 不验证代理是否可信，仅提取头信息
func GetForwardedIP(r *http.Request) string {
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

	return ""
}

// GetClientIP 获取客户端真实 IP
// 安全版本：只有当 RemoteAddr 是信任的代理时，才信任代理头
func GetClientIP(r *http.Request) string {
	directIP := GetDirectIP(r)

	// 如果直连 IP 是信任的代理，使用代理头中的客户端 IP
	if IsTrustedProxy(directIP) {
		if forwardedIP := GetForwardedIP(r); forwardedIP != "" {
			return forwardedIP
		}
	}

	// 否则使用直连 IP
	return directIP
}

// GetClientIPs 获取客户端的两个 IP（用于审计日志）
// 返回：直连 IP 和代理转发的 IP（如果有）
func GetClientIPs(r *http.Request) (directIP, forwardedIP string) {
	directIP = GetDirectIP(r)
	forwardedIP = GetForwardedIP(r)
	return
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

// GetCloudflareIPRangesString 获取 Cloudflare IP 范围字符串
// 用于前端预设按钮
func GetCloudflareIPRangesString() string {
	return strings.Join(CloudflareIPRanges, ",")
}
