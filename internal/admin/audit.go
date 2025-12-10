package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"sss/internal/storage"
	"sss/internal/utils"
)

// AuditLogResponse 审计日志响应
type AuditLogResponse struct {
	Logs  []storage.AuditLog `json:"logs"`
	Total int                `json:"total"`
	Limit int                `json:"limit"`
	Page  int                `json:"page"`
}

// handleAuditLogs 处理审计日志查询
func (h *Handler) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	query := &storage.AuditLogQuery{}

	// 解析查询参数
	if action := r.URL.Query().Get("action"); action != "" {
		query.Action = storage.AuditAction(action)
	}
	if actor := r.URL.Query().Get("actor"); actor != "" {
		query.Actor = actor
	}
	if ip := r.URL.Query().Get("ip"); ip != "" {
		query.IP = ip
	}
	if resource := r.URL.Query().Get("resource"); resource != "" {
		query.Resource = resource
	}
	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			query.StartTime = &t
		}
	}
	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			query.EndTime = &t
		}
	}
	if success := r.URL.Query().Get("success"); success != "" {
		b := success == "true" || success == "1"
		query.Success = &b
	}

	// 分页参数
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	query.Limit = 50
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil && v > 0 && v <= 100 {
			query.Limit = v
		}
	}
	query.Offset = (page - 1) * query.Limit

	logs, total, err := h.metadata.QueryAuditLogs(query)
	if err != nil {
		utils.Error("查询审计日志失败", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, AuditLogResponse{
		Logs:  logs,
		Total: total,
		Limit: query.Limit,
		Page:  page,
	})
}

// handleAuditStats 获取审计统计
func (h *Handler) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	stats, err := h.metadata.GetAuditStats()
	if err != nil {
		utils.Error("获取审计统计失败", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}

	utils.WriteJSONResponse(w, stats)
}

// Audit 记录审计日志的辅助方法
// 同时记录直连 IP (RemoteAddr) 和代理转发的 IP (X-Forwarded-For 等)
// 如果启用了 GeoIP，还会记录地理位置信息
func (h *Handler) Audit(r *http.Request, action storage.AuditAction, actor, resource string, success bool, detail interface{}) {
	var detailStr string
	if detail != nil {
		if str, ok := detail.(string); ok {
			detailStr = str
		} else {
			if bytes, err := json.Marshal(detail); err == nil {
				detailStr = string(bytes)
			}
		}
	}

	// 获取双 IP：直连 IP 和代理转发的 IP
	directIP, forwardedIP := utils.GetClientIPs(r)

	// 查询地理位置（优先使用客户端真实 IP）
	var location string
	geoIP := utils.GetGeoIPService()
	if geoIP.IsEnabled() {
		// 优先使用 GetClientIP 返回的真实客户端 IP
		clientIP := utils.GetClientIP(r)
		location = geoIP.LookupString(clientIP)
	}

	log := &storage.AuditLog{
		Timestamp:   time.Now().UTC(),
		Action:      action,
		Actor:       actor,
		IP:          directIP,
		ForwardedIP: forwardedIP,
		Location:    location,
		Resource:    resource,
		Detail:      detailStr,
		Success:     success,
		UserAgent:   utils.GetUserAgent(r),
	}

	if err := h.metadata.WriteAuditLog(log); err != nil {
		utils.Error("写入审计日志失败", "error", err, "action", action)
	}
}
