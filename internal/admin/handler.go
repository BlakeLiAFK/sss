package admin

import (
	"net/http"
	"strings"

	"sss/internal/storage"
	"sss/internal/utils"
)

// Handler 管理后台处理器
type Handler struct {
	metadata  *storage.MetadataStore
	filestore *storage.FileStore
}

// NewHandler 创建管理后台处理器
func NewHandler(metadata *storage.MetadataStore, filestore *storage.FileStore) *Handler {
	return &Handler{
		metadata:  metadata,
		filestore: filestore,
	}
}

// ServeHTTP 处理管理后台请求
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 安装相关 API（无需认证）
	if strings.HasPrefix(path, "/api/setup") {
		h.handleSetupAPI(w, r)
		return
	}

	// 登录 API（无需认证）
	if path == "/api/admin/login" {
		h.handleAdminLogin(w, r)
		return
	}

	// 其他 API 需要认证
	if !h.checkAdminAuth(r) {
		utils.WriteErrorResponse(w, "Unauthorized", "未授权访问", http.StatusUnauthorized)
		return
	}

	// 路由分发
	h.route(w, r)
}

// route 路由分发
func (h *Handler) route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/")

	switch {
	case path == "logout":
		h.handleAdminLogout(w, r)
	case path == "apikeys":
		h.handleAPIKeys(w, r)
	case strings.HasPrefix(path, "apikeys/"):
		h.handleAPIKeyDetail(w, r, strings.TrimPrefix(path, "apikeys/"))
	case path == "buckets":
		h.handleAdminBucketsAPI(w, r)
	case strings.HasPrefix(path, "buckets/"):
		h.handleAdminBucketOps(w, r, strings.TrimPrefix(path, "buckets/"))
	case path == "stats/overview":
		h.handleStorageStats(w, r)
	case path == "stats/recent":
		h.handleRecentObjects(w, r)
	case path == "storage/gc":
		h.handleGC(w, r)
	case path == "storage/integrity":
		h.handleIntegrity(w, r)
	case path == "migrate":
		h.handleMigrateAPI(w, r)
	case strings.HasPrefix(path, "migrate/"):
		h.handleMigrateJob(w, r, strings.TrimPrefix(path, "migrate/"))
	case path == "audit":
		h.handleAuditLogs(w, r)
	case path == "audit/stats":
		h.handleAuditStats(w, r)
	case path == "settings":
		h.handleSettings(w, r)
	case path == "settings/password":
		h.handleChangePassword(w, r)
	case path == "settings/geoip":
		h.handleGeoIP(w, r)
	default:
		utils.WriteErrorResponse(w, "NotFound", "API endpoint not found", http.StatusNotFound)
	}
}
