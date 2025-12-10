package storage

import (
	"database/sql"
	"sync"
	"time"
)

// GeoStatEntry 地理位置统计条目
type GeoStatEntry struct {
	ID           int64     `json:"id"`
	Date         string    `json:"date"`          // 日期 YYYY-MM-DD
	CountryCode  string    `json:"country_code"`  // 国家代码
	Country      string    `json:"country"`       // 国家名称
	City         string    `json:"city"`          // 城市名称
	Region       string    `json:"region"`        // 省/州
	RequestCount int64     `json:"request_count"` // 请求次数
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GeoStatsConfig GeoStats 配置
type GeoStatsConfig struct {
	Enabled       bool   `json:"enabled"`        // 是否启用
	Mode          string `json:"mode"`           // 写入模式: realtime/batch
	BatchSize     int    `json:"batch_size"`     // 批量模式缓存大小
	FlushInterval int    `json:"flush_interval"` // 批量模式刷新间隔（秒）
	RetentionDays int    `json:"retention_days"` // 数据保留天数
}

// GeoStatsKey 统计聚合键
type GeoStatsKey struct {
	Date        string
	CountryCode string
	City        string
}

// GeoStatsValue 统计聚合值
type GeoStatsValue struct {
	Country      string
	Region       string
	RequestCount int64
}

// GeoStatsService GeoStats 服务
type GeoStatsService struct {
	mu       sync.Mutex
	store    *MetadataStore
	config   *GeoStatsConfig
	buffer   map[GeoStatsKey]*GeoStatsValue
	stopChan chan struct{}
	ticker   *time.Ticker
	running  bool
}

var (
	geoStatsService     *GeoStatsService
	geoStatsServiceOnce sync.Once
)

// GetGeoStatsService 获取 GeoStats 服务单例
func GetGeoStatsService() *GeoStatsService {
	geoStatsServiceOnce.Do(func() {
		geoStatsService = &GeoStatsService{
			buffer: make(map[GeoStatsKey]*GeoStatsValue),
			config: &GeoStatsConfig{
				Enabled:       false,
				Mode:          "realtime",
				BatchSize:     100,
				FlushInterval: 60,
				RetentionDays: 90,
			},
		}
	})
	return geoStatsService
}

// InitGeoStatsService 初始化 GeoStats 服务
func InitGeoStatsService(store *MetadataStore) {
	service := GetGeoStatsService()
	service.mu.Lock()
	defer service.mu.Unlock()

	service.store = store
	service.loadConfig()

	// 如果启用且是批量模式，启动后台刷新
	if service.config.Enabled && service.config.Mode == "batch" {
		service.startBatchFlush()
	}
}

// loadConfig 从数据库加载配置
func (s *GeoStatsService) loadConfig() {
	if s.store == nil {
		return
	}

	if enabled, err := s.store.GetSetting(SettingGeoStatsEnabled); err == nil && enabled == "true" {
		s.config.Enabled = true
	}

	if mode, err := s.store.GetSetting(SettingGeoStatsMode); err == nil && mode != "" {
		s.config.Mode = mode
	}

	if batchSize, err := s.store.GetSetting(SettingGeoStatsBatchSize); err == nil && batchSize != "" {
		var size int
		if _, err := parseIntSafe(batchSize, &size); err == nil && size > 0 {
			s.config.BatchSize = size
		}
	}

	if flushInterval, err := s.store.GetSetting(SettingGeoStatsFlushInterval); err == nil && flushInterval != "" {
		var interval int
		if _, err := parseIntSafe(flushInterval, &interval); err == nil && interval > 0 {
			s.config.FlushInterval = interval
		}
	}

	if retentionDays, err := s.store.GetSetting(SettingGeoStatsRetentionDays); err == nil && retentionDays != "" {
		var days int
		if _, err := parseIntSafe(retentionDays, &days); err == nil && days > 0 {
			s.config.RetentionDays = days
		}
	}
}

// GetConfig 获取当前配置
func (s *GeoStatsService) GetConfig() GeoStatsConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.config
}

// UpdateConfig 更新配置
func (s *GeoStatsService) UpdateConfig(cfg GeoStatsConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查模式变化，如果从批量切换到实时，先刷新缓冲区
	if s.config.Mode == "batch" && cfg.Mode == "realtime" {
		s.flushBuffer()
		s.stopBatchFlush()
	}

	// 更新配置
	s.config = &cfg

	// 如果启用且是批量模式，启动后台刷新
	if s.config.Enabled && s.config.Mode == "batch" && !s.running {
		s.startBatchFlush()
	} else if (!s.config.Enabled || s.config.Mode != "batch") && s.running {
		s.stopBatchFlush()
	}

	return nil
}

// Record 记录一次请求
func (s *GeoStatsService) Record(countryCode, country, city, region string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.config.Enabled || s.store == nil {
		return
	}

	date := time.Now().UTC().Format("2006-01-02")

	if s.config.Mode == "realtime" {
		// 实时模式：直接写入数据库
		s.recordToDB(date, countryCode, country, city, region, 1)
	} else {
		// 批量模式：写入缓冲区
		key := GeoStatsKey{
			Date:        date,
			CountryCode: countryCode,
			City:        city,
		}

		if existing, ok := s.buffer[key]; ok {
			existing.RequestCount++
		} else {
			s.buffer[key] = &GeoStatsValue{
				Country:      country,
				Region:       region,
				RequestCount: 1,
			}
		}

		// 如果缓冲区达到阈值，立即刷新
		if len(s.buffer) >= s.config.BatchSize {
			s.flushBuffer()
		}
	}
}

// recordToDB 写入数据库
func (s *GeoStatsService) recordToDB(date, countryCode, country, city, region string, count int64) {
	if s.store == nil {
		return
	}

	s.store.withWriteLock(func() error {
		_, err := s.store.db.Exec(`
			INSERT INTO geo_stats (date, country_code, country, city, region, request_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(date, country_code, city) DO UPDATE SET
				request_count = request_count + ?,
				updated_at = CURRENT_TIMESTAMP
		`, date, countryCode, country, city, region, count, count)
		return err
	})
}

// flushBuffer 刷新缓冲区到数据库
func (s *GeoStatsService) flushBuffer() {
	if len(s.buffer) == 0 {
		return
	}

	for key, value := range s.buffer {
		s.recordToDB(key.Date, key.CountryCode, value.Country, key.City, value.Region, value.RequestCount)
	}

	// 清空缓冲区
	s.buffer = make(map[GeoStatsKey]*GeoStatsValue)
}

// Flush 手动刷新缓冲区（公开方法）
func (s *GeoStatsService) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushBuffer()
}

// startBatchFlush 启动批量刷新定时器
func (s *GeoStatsService) startBatchFlush() {
	if s.running {
		return
	}

	s.stopChan = make(chan struct{})
	s.ticker = time.NewTicker(time.Duration(s.config.FlushInterval) * time.Second)
	s.running = true

	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.Flush()
			case <-s.stopChan:
				return
			}
		}
	}()
}

// stopBatchFlush 停止批量刷新定时器
func (s *GeoStatsService) stopBatchFlush() {
	if !s.running {
		return
	}

	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.stopChan != nil {
		close(s.stopChan)
	}
	s.running = false
}

// Stop 停止服务（程序退出时调用）
func (s *GeoStatsService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 刷新缓冲区
	s.flushBuffer()
	// 停止定时器
	s.stopBatchFlush()
}

// IsEnabled 检查是否启用
func (s *GeoStatsService) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Enabled
}

// initGeoStatsTable 初始化 geo_stats 表
func (m *MetadataStore) initGeoStatsTable() error {
	schema := `CREATE TABLE IF NOT EXISTS geo_stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		country_code TEXT NOT NULL DEFAULT '',
		country TEXT NOT NULL DEFAULT '',
		city TEXT NOT NULL DEFAULT '',
		region TEXT NOT NULL DEFAULT '',
		request_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(date, country_code, city)
	)`
	if _, err := m.db.Exec(schema); err != nil {
		return err
	}

	// 创建索引
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_geo_stats_date ON geo_stats(date)`,
		`CREATE INDEX IF NOT EXISTS idx_geo_stats_country ON geo_stats(country_code)`,
	}
	for _, idx := range indexes {
		if _, err := m.db.Exec(idx); err != nil {
			return err
		}
	}

	return nil
}

// GetGeoStats 获取地理位置统计数据
func (m *MetadataStore) GetGeoStats(startDate, endDate string, limit int) ([]GeoStatEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	query := `
		SELECT id, date, country_code, country, city, region, request_count, created_at, updated_at
		FROM geo_stats
		WHERE date >= ? AND date <= ?
		ORDER BY request_count DESC
		LIMIT ?
	`

	rows, err := m.db.Query(query, startDate, endDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []GeoStatEntry
	for rows.Next() {
		var e GeoStatEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.CountryCode, &e.Country, &e.City, &e.Region, &e.RequestCount, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetGeoStatsAggregated 获取聚合的地理位置统计（按国家或城市聚合）
func (m *MetadataStore) GetGeoStatsAggregated(startDate, endDate, groupBy string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var query string
	switch groupBy {
	case "city":
		query = `
			SELECT country_code, country, city, region, SUM(request_count) as total
			FROM geo_stats
			WHERE date >= ? AND date <= ?
			GROUP BY country_code, city
			ORDER BY total DESC
			LIMIT ?
		`
	default: // country
		query = `
			SELECT country_code, country, SUM(request_count) as total
			FROM geo_stats
			WHERE date >= ? AND date <= ?
			GROUP BY country_code
			ORDER BY total DESC
			LIMIT ?
		`
	}

	rows, err := m.db.Query(query, startDate, endDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		result := make(map[string]interface{})
		if groupBy == "city" {
			var countryCode, country, city, region string
			var total int64
			if err := rows.Scan(&countryCode, &country, &city, &region, &total); err != nil {
				return nil, err
			}
			result["country_code"] = countryCode
			result["country"] = country
			result["city"] = city
			result["region"] = region
			result["total"] = total
		} else {
			var countryCode, country string
			var total int64
			if err := rows.Scan(&countryCode, &country, &total); err != nil {
				return nil, err
			}
			result["country_code"] = countryCode
			result["country"] = country
			result["total"] = total
		}
		results = append(results, result)
	}
	return results, nil
}

// GetGeoStatsSummary 获取统计摘要
func (m *MetadataStore) GetGeoStatsSummary(startDate, endDate string) (map[string]interface{}, error) {
	summary := make(map[string]interface{})

	// 总请求数
	var totalRequests sql.NullInt64
	err := m.db.QueryRow(`
		SELECT SUM(request_count)
		FROM geo_stats
		WHERE date >= ? AND date <= ?
	`, startDate, endDate).Scan(&totalRequests)
	if err != nil {
		return nil, err
	}
	summary["total_requests"] = totalRequests.Int64

	// 国家数
	var countryCount int
	err = m.db.QueryRow(`
		SELECT COUNT(DISTINCT country_code)
		FROM geo_stats
		WHERE date >= ? AND date <= ? AND country_code != ''
	`, startDate, endDate).Scan(&countryCount)
	if err != nil {
		return nil, err
	}
	summary["country_count"] = countryCount

	// 城市数
	var cityCount int
	err = m.db.QueryRow(`
		SELECT COUNT(DISTINCT city)
		FROM geo_stats
		WHERE date >= ? AND date <= ? AND city != ''
	`, startDate, endDate).Scan(&cityCount)
	if err != nil {
		return nil, err
	}
	summary["city_count"] = cityCount

	return summary, nil
}

// DeleteGeoStats 删除指定日期范围的统计数据
func (m *MetadataStore) DeleteGeoStats(beforeDate string) (int64, error) {
	var affected int64
	err := m.withWriteLock(func() error {
		result, err := m.db.Exec("DELETE FROM geo_stats WHERE date < ?", beforeDate)
		if err != nil {
			return err
		}
		affected, _ = result.RowsAffected()
		return nil
	})
	return affected, err
}

// ClearGeoStats 清空所有统计数据
func (m *MetadataStore) ClearGeoStats() error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec("DELETE FROM geo_stats")
		return err
	})
}

// CleanupOldGeoStats 清理过期的统计数据
func (m *MetadataStore) CleanupOldGeoStats(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02")
	return m.DeleteGeoStats(cutoffDate)
}
