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
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
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

	for name, dest := range map[string]**time.Time{"start_time": &query.StartTime, "end_time": &query.EndTime} {
		if value := r.URL.Query().Get(name); value != "" {
			stamp, err := time.Parse(time.RFC3339, value)
			if err != nil {
				writeAdminError(w, utils.ErrInvalidArgument, 400, "")
				return
			}
			*dest = &stamp
		}
	}
	if query.StartTime != nil && query.EndTime != nil && query.StartTime.After(*query.EndTime) {
		writeAdminError(w, utils.ErrInvalidArgument, 400, "")
		return
	}
	if value := r.URL.Query().Get("success"); value != "" {
		b, err := strconv.ParseBool(value)
		if err != nil {
			writeAdminError(w, utils.ErrInvalidArgument, 400, "")
			return
		}
		query.Success = &b
	}
	page := 1
	query.Limit = 50
	for name, dest := range map[string]*int{"page": &page, "limit": &query.Limit} {
		if value := r.URL.Query().Get(name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 1000000 || (name == "limit" && n > 100) {
				writeAdminError(w, utils.ErrInvalidArgument, 400, "")
				return
			}
			*dest = n
		}
	}
	query.Offset = (page - 1) * query.Limit

	logs, total, err := h.metadata.QueryAuditLogs(query)
	if err != nil {
		utils.Error("查询审计日志失败", "error", err)
		writeAdminError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
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
		writeAdminError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	stats, err := h.metadata.GetAuditStats()
	if err != nil {
		utils.Error("获取审计统计失败", "error", err)
		writeAdminError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
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
