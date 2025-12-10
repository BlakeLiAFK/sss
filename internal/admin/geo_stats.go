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
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
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

	// 更新启用状态
	if req.Enabled != nil {
		// 如果要启用 GeoStats，必须先启用 GeoIP
		if *req.Enabled && !geoIPEnabled {
			utils.WriteErrorResponse(w, "DependencyError", "GeoStats requires GeoIP to be enabled first", http.StatusBadRequest)
			return
		}
		cfg.Enabled = *req.Enabled
		enabledStr := "false"
		if cfg.Enabled {
			enabledStr = "true"
		}
		if err := h.metadata.SetSetting(storage.SettingGeoStatsEnabled, enabledStr); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.GeoStats.Enabled = cfg.Enabled
	}

	// 更新写入模式
	if req.Mode != nil && *req.Mode != "" {
		mode := *req.Mode
		if mode != "realtime" && mode != "batch" {
			utils.WriteErrorResponse(w, "InvalidParameter", "mode must be 'realtime' or 'batch'", http.StatusBadRequest)
			return
		}
		cfg.Mode = mode
		if err := h.metadata.SetSetting(storage.SettingGeoStatsMode, mode); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.GeoStats.Mode = mode
	}

	// 更新批量模式缓存大小
	if req.BatchSize != nil && *req.BatchSize > 0 {
		cfg.BatchSize = *req.BatchSize
		if err := h.metadata.SetSetting(storage.SettingGeoStatsBatchSize, strconv.Itoa(cfg.BatchSize)); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.GeoStats.BatchSize = cfg.BatchSize
	}

	// 更新刷新间隔
	if req.FlushInterval != nil && *req.FlushInterval > 0 {
		cfg.FlushInterval = *req.FlushInterval
		if err := h.metadata.SetSetting(storage.SettingGeoStatsFlushInterval, strconv.Itoa(cfg.FlushInterval)); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.GeoStats.FlushInterval = cfg.FlushInterval
	}

	// 更新数据保留天数
	if req.RetentionDays != nil && *req.RetentionDays > 0 {
		cfg.RetentionDays = *req.RetentionDays
		if err := h.metadata.SetSetting(storage.SettingGeoStatsRetentionDays, strconv.Itoa(cfg.RetentionDays)); err != nil {
			utils.WriteErrorResponse(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}
		config.Global.GeoStats.RetentionDays = cfg.RetentionDays
	}

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
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
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

	// 解析分组方式
	groupBy := query.Get("group_by")

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
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
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
