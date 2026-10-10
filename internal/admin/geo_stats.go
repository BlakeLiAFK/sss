package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// GeoStatsConfigRequest GeoStats 配置请求
type GeoStatsConfigRequest struct {
	Enabled       *bool   `json:"enabled,omitempty"`
	Mode          *string `json:"mode,omitempty"`
	BatchSize     *int    `json:"batch_size,omitempty"`
	FlushInterval *int    `json:"flush_interval,omitempty"`
	RetentionDays *int    `json:"retention_days,omitempty"`
}

// GeoStatsConfigResponse GeoStats 配置响应
type GeoStatsConfigResponse struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`
	BatchSize     int    `json:"batch_size"`
	FlushInterval int    `json:"flush_interval"`
	RetentionDays int    `json:"retention_days"`
	GeoIPEnabled  bool   `json:"geoip_enabled"` // GeoIP 是否启用（依赖）
}

// handleGeoStatsConfig 处理 GeoStats 配置 API
func (h *Handler) handleGeoStatsConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getGeoStatsConfig(w, r)
	case http.MethodPut:
		h.updateGeoStatsConfig(w, r)
	default:
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// getGeoStatsConfig 获取 GeoStats 配置
func (h *Handler) getGeoStatsConfig(w http.ResponseWriter, r *http.Request) {
	service := storage.GetGeoStatsService()
	cfg := service.GetConfig()

	resp := GeoStatsConfigResponse{
		Enabled:       cfg.Enabled,
		Mode:          cfg.Mode,
		BatchSize:     cfg.BatchSize,
		FlushInterval: cfg.FlushInterval,
		RetentionDays: cfg.RetentionDays,
		GeoIPEnabled:  utils.GetGeoIPService().IsEnabled(),
	}

	utils.WriteJSONResponse(w, resp)
}

// updateGeoStatsConfig 更新 GeoStats 配置
func (h *Handler) updateGeoStatsConfig(w http.ResponseWriter, r *http.Request) {
	var req GeoStatsConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, "InvalidRequest", "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 获取当前配置
	service := storage.GetGeoStatsService()
	cfg := service.GetConfig()

	// 检查 GeoIP 依赖
	geoIPEnabled := utils.GetGeoIPService().IsEnabled()

	values := map[string]string{}
	if req.Enabled != nil {
		if *req.Enabled && !geoIPEnabled {
			writeAdminError(w, utils.ErrInvalidArgument, 400, "")
			return
		}
		cfg.Enabled = *req.Enabled
		values[storage.SettingGeoStatsEnabled] = strconv.FormatBool(*req.Enabled)
	}
	if req.Mode != nil {
		if *req.Mode != "realtime" && *req.Mode != "batch" {
			writeAdminError(w, utils.ErrInvalidArgument, 400, "")
			return
		}
		cfg.Mode = *req.Mode
		values[storage.SettingGeoStatsMode] = *req.Mode
	}
	for key, item := range map[string]struct {
		value  *int
		target *int
		max    int
	}{
		storage.SettingGeoStatsBatchSize: {req.BatchSize, &cfg.BatchSize, 10000}, storage.SettingGeoStatsFlushInterval: {req.FlushInterval, &cfg.FlushInterval, 86400}, storage.SettingGeoStatsRetentionDays: {req.RetentionDays, &cfg.RetentionDays, 36500},
	} {
		if item.value != nil {
			if *item.value < 1 || *item.value > item.max {
				writeAdminError(w, utils.ErrInvalidArgument, 400, "")
				return
			}
			*item.target = *item.value
			values[key] = strconv.Itoa(*item.value)
		}
	}
	if err := h.metadata.SetSettings(values); err != nil {
		writeAdminError(w, utils.ErrInternalError, 500, "")
		return
	}
	config.Global.GeoStats.Enabled = cfg.Enabled
	config.Global.GeoStats.Mode = cfg.Mode
	config.Global.GeoStats.BatchSize = cfg.BatchSize
	config.Global.GeoStats.FlushInterval = cfg.FlushInterval
	config.Global.GeoStats.RetentionDays = cfg.RetentionDays
	// 应用配置到服务
	if err := service.UpdateConfig(cfg); err != nil {
		utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录审计日志
	h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "geo_stats", true, "GeoStats configuration updated")

	// 返回更新后的配置
	h.getGeoStatsConfig(w, r)
}

// GeoStatsDataResponse GeoStats 数据响应
type GeoStatsDataResponse struct {
	Data       []storage.GeoStatEntry `json:"data"`
	StartDate  string                 `json:"start_date"`
	EndDate    string                 `json:"end_date"`
	TotalCount int                    `json:"total_count"`
}

// GeoStatsAggregatedResponse GeoStats 聚合数据响应
type GeoStatsAggregatedResponse struct {
	Data      []map[string]interface{} `json:"data"`
	GroupBy   string                   `json:"group_by"`
	StartDate string                   `json:"start_date"`
	EndDate   string                   `json:"end_date"`
}

// handleGeoStatsData 处理 GeoStats 数据 API
func (h *Handler) handleGeoStatsData(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getGeoStatsData(w, r)
	case http.MethodDelete:
		h.deleteGeoStatsData(w, r)
	default:
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
	}
}

// getGeoStatsData 获取 GeoStats 数据
func (h *Handler) getGeoStatsData(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// 解析日期范围
	startDate := query.Get("start_date")
	endDate := query.Get("end_date")

	// 默认最近 30 天
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	if !validDateRange(startDate, endDate) {
		writeAdminError(w, utils.ErrInvalidArgument, 400, "")
		return
	}
	// 解析分组方式
	groupBy := query.Get("group_by")
	if groupBy != "" && groupBy != "country" && groupBy != "city" {
		writeAdminError(w, utils.ErrInvalidArgument, 400, "")
		return
	}

	// 解析限制数量
	limit := 100
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if groupBy != "" {
		// 返回聚合数据
		data, err := h.metadata.GetGeoStatsAggregated(startDate, endDate, groupBy, limit)
		if err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}

		resp := GeoStatsAggregatedResponse{
			Data:      data,
			GroupBy:   groupBy,
			StartDate: startDate,
			EndDate:   endDate,
		}
		utils.WriteJSONResponse(w, resp)
	} else {
		// 返回原始数据
		data, err := h.metadata.GetGeoStats(startDate, endDate, limit)
		if err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}

		resp := GeoStatsDataResponse{
			Data:       data,
			StartDate:  startDate,
			EndDate:    endDate,
			TotalCount: len(data),
		}
		utils.WriteJSONResponse(w, resp)
	}
}

// deleteGeoStatsData 删除 GeoStats 数据
func (h *Handler) deleteGeoStatsData(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// 检查是否清空所有数据
	if query.Get("all") == "true" {
		if err := h.metadata.ClearGeoStats(); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}

		// 记录审计日志
		h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "geo_stats", true, "All GeoStats data cleared")

		utils.WriteJSONResponse(w, map[string]interface{}{
			"success": true,
			"message": "All geo stats data cleared",
		})
		return
	}

	// 按日期删除
	beforeDate := query.Get("before_date")
	if beforeDate == "" {
		// 默认清理超过保留期的数据
		service := storage.GetGeoStatsService()
		cfg := service.GetConfig()
		affected, err := h.metadata.CleanupOldGeoStats(cfg.RetentionDays)
		if err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}

		// 记录审计日志
		h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "geo_stats", true, "Cleanup old GeoStats data")

		utils.WriteJSONResponse(w, map[string]interface{}{
			"success":  true,
			"message":  "Old geo stats data cleaned up",
			"affected": affected,
		})
		return
	}

	if _, err := time.Parse("2006-01-02", beforeDate); err != nil {
		writeAdminError(w, utils.ErrInvalidArgument, 400, "")
		return
	}
	// 删除指定日期之前的数据
	affected, err := h.metadata.DeleteGeoStats(beforeDate)
	if err != nil {
		utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录审计日志
	h.Audit(r, storage.AuditActionSettingsUpdate, "admin", "geo_stats", true, "Delete GeoStats data before "+beforeDate)

	utils.WriteJSONResponse(w, map[string]interface{}{
		"success":  true,
		"message":  "Geo stats data deleted",
		"affected": affected,
	})
}

// handleGeoStatsSummary 处理 GeoStats 摘要 API
func (h *Handler) handleGeoStatsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	query := r.URL.Query()

	// 解析日期范围
	startDate := query.Get("start_date")
	endDate := query.Get("end_date")

	// 默认最近 30 天
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	if !validDateRange(startDate, endDate) {
		writeAdminError(w, utils.ErrInvalidArgument, 400, "")
		return
	}
	summary, err := h.metadata.GetGeoStatsSummary(startDate, endDate)
	if err != nil {
		utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加日期范围到响应
	summary["start_date"] = startDate
	summary["end_date"] = endDate

	utils.WriteJSONResponse(w, summary)
}

func validDateRange(start, end string) bool {
	a, e1 := time.Parse("2006-01-02", start)
	b, e2 := time.Parse("2006-01-02", end)
	return e1 == nil && e2 == nil && !a.After(b)
}
