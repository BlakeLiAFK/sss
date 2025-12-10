package utils

import (
	"net/netip"
	"os"
	"path/filepath"
	"sync"

	"github.com/oschwald/geoip2-golang/v2"
)

// GeoIPResult 地理位置查询结果
type GeoIPResult struct {
	Country     string `json:"country,omitempty"`      // 国家
	CountryCode string `json:"country_code,omitempty"` // 国家代码 (ISO 3166-1)
	City        string `json:"city,omitempty"`         // 城市
	Region      string `json:"region,omitempty"`       // 省/州
}

// GeoIPService GeoIP 服务
type GeoIPService struct {
	mu       sync.RWMutex
	db       *geoip2.Reader
	dbPath   string
	enabled  bool
	lastLoad string // 上次加载的文件路径
}

var (
	geoIPService     *GeoIPService
	geoIPServiceOnce sync.Once
)

// GetGeoIPService 获取 GeoIP 服务单例
func GetGeoIPService() *GeoIPService {
	geoIPServiceOnce.Do(func() {
		geoIPService = &GeoIPService{}
	})
	return geoIPService
}

// Load 加载 GeoIP 数据库
func (s *GeoIPService) Load(dbPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已有数据库，先关闭
	if s.db != nil {
		s.db.Close()
		s.db = nil
		s.enabled = false
	}

	// 检查文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		Info("GeoIP 数据库不存在，功能禁用", "path", dbPath)
		return nil
	}

	// 打开数据库
	db, err := geoip2.Open(dbPath)
	if err != nil {
		Error("加载 GeoIP 数据库失败", "error", err, "path", dbPath)
		return err
	}

	s.db = db
	s.dbPath = dbPath
	s.enabled = true
	s.lastLoad = dbPath
	Info("GeoIP 数据库已加载", "path", dbPath)
	return nil
}

// Reload 重新加载数据库
func (s *GeoIPService) Reload() error {
	if s.lastLoad != "" {
		return s.Load(s.lastLoad)
	}
	return nil
}

// Close 关闭数据库
func (s *GeoIPService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		s.db.Close()
		s.db = nil
		s.enabled = false
	}
}

// IsEnabled 是否启用
func (s *GeoIPService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// Lookup 查询 IP 地理位置
func (s *GeoIPService) Lookup(ipStr string) *GeoIPResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled || s.db == nil {
		return nil
	}

	// 解析 IP
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil
	}

	// 查询城市数据库
	record, err := s.db.City(ip)
	if err != nil || !record.HasData() {
		return nil
	}

	result := &GeoIPResult{}

	// 国家信息 (v2 API: ISOCode 而非 IsoCode, Names 是 struct 而非 map)
	if record.Country.ISOCode != "" {
		result.CountryCode = record.Country.ISOCode
		// 优先使用中文名称
		if record.Country.Names.SimplifiedChinese != "" {
			result.Country = record.Country.Names.SimplifiedChinese
		} else if record.Country.Names.English != "" {
			result.Country = record.Country.Names.English
		}
	}

	// 城市信息
	if record.City.Names.SimplifiedChinese != "" {
		result.City = record.City.Names.SimplifiedChinese
	} else if record.City.Names.English != "" {
		result.City = record.City.Names.English
	}

	// 省/州信息
	if len(record.Subdivisions) > 0 {
		sub := record.Subdivisions[0]
		if sub.Names.SimplifiedChinese != "" {
			result.Region = sub.Names.SimplifiedChinese
		} else if sub.Names.English != "" {
			result.Region = sub.Names.English
		}
	}

	return result
}

// LookupString 查询 IP 地理位置，返回格式化字符串
func (s *GeoIPService) LookupString(ipStr string) string {
	result := s.Lookup(ipStr)
	if result == nil {
		return ""
	}

	// 格式化输出：国家 省份 城市
	var parts []string
	if result.Country != "" {
		parts = append(parts, result.Country)
	}
	if result.Region != "" && result.Region != result.City {
		parts = append(parts, result.Region)
	}
	if result.City != "" {
		parts = append(parts, result.City)
	}

	if len(parts) == 0 {
		return ""
	}

	// 使用空格连接
	location := ""
	for i, part := range parts {
		if i > 0 {
			location += " "
		}
		location += part
	}
	return location
}

// GetDefaultGeoIPPath 获取默认的 GeoIP 数据库路径
func GetDefaultGeoIPPath(dataPath string) string {
	return filepath.Join(dataPath, "geoip", "GeoIP.mmdb")
}

// InitGeoIP 初始化 GeoIP 服务
func InitGeoIP(dataPath string) {
	dbPath := GetDefaultGeoIPPath(dataPath)
	GetGeoIPService().Load(dbPath)
}
