package api

import (
	"net/http"
	"strings"
	"time"

	"sss/internal/auth"
	"sss/internal/storage"
	"sss/internal/utils"
)

// Server S3服务器
type Server struct {
	metadata  *storage.MetadataStore
	filestore *storage.FileStore
	mux       *http.ServeMux
}

// NewServer 创建服务器
func NewServer(metadata *storage.MetadataStore, filestore *storage.FileStore) *Server {
	s := &Server{
		metadata:  metadata,
		filestore: filestore,
		mux:       http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/", s.handleRequest)
	// Web管理界面API端点
	s.mux.HandleFunc("/api/presign", s.handlePresign)
	s.mux.HandleFunc("/api/bucket/", s.handleBucketAPI)
}

// ServeHTTP 实现 http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 添加通用头部
	w.Header().Set("Server", "SSS")
	w.Header().Set("x-amz-request-id", utils.GenerateRequestID())

	// CORS 支持
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "ETag, x-amz-request-id, x-amz-id-2")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	utils.Info("request", "method", r.Method, "path", r.URL.Path, "query", r.URL.RawQuery)

	s.mux.ServeHTTP(w, r)
}

// handleRequest 处理请求
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 1. 检查是否是静态文件请求
	// 对于根路径，优先检查是否有 S3 签名头，有则处理为 API 请求
	if r.URL.Path == "/" {
		// 优先检查 Authorization 头或预签名参数，这是 S3 API 请求的标志
		hasS3Auth := r.Header.Get("Authorization") != "" || r.URL.Query().Get("X-Amz-Signature") != ""
		if !hasS3Auth {
			accept := r.Header.Get("Accept")
			userAgent := r.Header.Get("User-Agent")
			// 如果 Accept 包含 text/html 或者 User-Agent 包含浏览器关键字
			if (accept != "" && strings.Contains(accept, "text/html")) ||
				(userAgent != "" && (strings.Contains(userAgent, "Mozilla") || strings.Contains(userAgent, "Chrome") || strings.Contains(userAgent, "Safari") || strings.Contains(userAgent, "Firefox"))) {
				// 浏览器访问，返回 HTML
				s.serveStatic(w, r)
				return
			}
		}
		// 否则继续处理 S3 API
	} else if strings.HasPrefix(r.URL.Path, "/assets/") {
		s.serveStatic(w, r)
		return
	}

	// 2. 检查是否是API管理路径
	if strings.HasPrefix(r.URL.Path, "/api/") {
		// API路径需要认证
		if !s.checkAuth(r, w) {
			return
		}
		// 交给API处理器
		if strings.HasPrefix(r.URL.Path, "/api/presign") {
			s.handlePresign(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/bucket/") {
			s.handleBucketAPI(w, r)
			return
		}
	}

	// 3. S3 API 处理
	// 解析路径获取bucket
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	var bucket string
	if len(parts) >= 1 && parts[0] != "" {
		bucket = parts[0]
	}

	// 4. 认证检查
	if bucket != "" {
		// 检查桶是否为公有（只对GET请求）
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if bucketInfo, err := s.metadata.GetBucket(bucket); err == nil && bucketInfo != nil && bucketInfo.IsPublic {
				// 公有桶的GET/HEAD请求跳过认证
				utils.Debug("public bucket access", "bucket", bucket, "method", r.Method)
			} else {
				// 私有桶或检查失败需要认证
				if !s.checkAuth(r, w) {
					return
				}
			}
		} else {
			// 非GET请求需要认证
			if !s.checkAuth(r, w) {
				return
			}
		}
	} else {
		// ListBuckets需要认证
		if !s.checkAuth(r, w) {
			return
		}
	}

	// 重新解析路径（之前的bucket已经获取了）
	key := ""
	if len(parts) >= 2 {
		key = parts[1]
	}

	// 检查是否是多段上传相关操作
	query := r.URL.Query()

	// 路由到具体处理器
	switch {
	// ListBuckets - GET /
	case r.Method == "GET" && bucket == "":
		s.handleListBuckets(w, r)

	// CreateBucket - PUT /{bucket}
	case r.Method == "PUT" && bucket != "" && key == "":
		s.handleCreateBucket(w, r, bucket)

	// DeleteBucket - DELETE /{bucket}
	case r.Method == "DELETE" && bucket != "" && key == "":
		s.handleDeleteBucket(w, r, bucket)

	// HeadBucket - HEAD /{bucket}
	case r.Method == "HEAD" && bucket != "" && key == "":
		s.handleHeadBucket(w, r, bucket)

	// ListObjects - GET /{bucket}
	case r.Method == "GET" && bucket != "" && key == "":
		s.handleListObjects(w, r, bucket)

	// Multipart Upload 操作
	case query.Get("uploads") != "":
		if r.Method == "POST" && key != "" {
			// InitiateMultipartUpload
			s.handleInitiateMultipartUpload(w, r, bucket, key)
		} else if r.Method == "GET" {
			// ListMultipartUploads (暂未实现)
			w.WriteHeader(http.StatusNotImplemented)
		}

	case query.Get("uploadId") != "":
		uploadID := query.Get("uploadId")
		switch r.Method {
		case "PUT":
			// UploadPart
			s.handleUploadPart(w, r, bucket, key, uploadID)
		case "POST":
			// CompleteMultipartUpload
			s.handleCompleteMultipartUpload(w, r, bucket, key, uploadID)
		case "DELETE":
			// AbortMultipartUpload
			s.handleAbortMultipartUpload(w, r, bucket, key, uploadID)
		case "GET":
			// ListParts
			s.handleListParts(w, r, bucket, key, uploadID)
		}

	// GetObject - GET /{bucket}/{key}
	case r.Method == "GET" && key != "":
		s.handleGetObject(w, r, bucket, key)

	// PutObject - PUT /{bucket}/{key}
	case r.Method == "PUT" && key != "":
		s.handlePutObject(w, r, bucket, key)

	// DeleteObject - DELETE /{bucket}/{key}
	case r.Method == "DELETE" && key != "":
		s.handleDeleteObject(w, r, bucket, key)

	// HeadObject - HEAD /{bucket}/{key}
	case r.Method == "HEAD" && key != "":
		s.handleHeadObject(w, r, bucket, key)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// PresignRequest 预签名请求结构
type PresignRequest struct {
	Method         string `json:"method"`
	Bucket         string `json:"bucket"`
	Key            string `json:"key"`
	ExpiresMinutes int    `json:"expiresMinutes"`
	MaxSizeMB      int64  `json:"maxSizeMB"`
	ContentType    string `json:"contentType"`
}

// PresignResponse 预签名响应结构
type PresignResponse struct {
	URL    string `json:"url"`
	Method string `json:"method"`
	Expires int   `json:"expires"`
}

// handlePresign 处理预签名URL生成请求
func (s *Server) handlePresign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	var req PresignRequest
	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
		return
	}

	// 验证请求参数
	if req.Bucket == "" || req.Key == "" {
		utils.WriteErrorResponse(w, "MissingRequiredParameter", "bucket and key are required", http.StatusBadRequest)
		return
	}

	// 检查存储桶是否存在
	bucket, err := s.metadata.GetBucket(req.Bucket)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	if bucket == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "")
		return
	}

	// 设置默认值
	if req.Method == "" {
		req.Method = "PUT"
	}
	if req.ExpiresMinutes == 0 {
		req.ExpiresMinutes = 60 // 默认1小时
	}
	if req.ExpiresMinutes > 7*24*60 { // 最大7天
		req.ExpiresMinutes = 7 * 24 * 60
	}

	// 构建预签名选项
	opts := &auth.PresignOptions{
		Expires: time.Duration(req.ExpiresMinutes) * time.Minute,
	}

	// 设置大小限制（MB转字节）
	if req.MaxSizeMB > 0 {
		opts.MaxContentLength = req.MaxSizeMB * 1024 * 1024
	}

	// 设置内容类型
	if req.ContentType != "" {
		opts.ContentType = req.ContentType
	}

	// 生成预签名URL
	url := auth.GeneratePresignedURLWithOptions(req.Method, req.Bucket, req.Key, opts)

	// 构建响应
	resp := PresignResponse{
		URL:      url,
		Method:   req.Method,
		Expires:  req.ExpiresMinutes * 60, // 转换为秒
	}

	utils.WriteJSONResponse(w, resp)
}

// BucketPublicRequest 设置桶公有/私有请求
type BucketPublicRequest struct {
	IsPublic bool `json:"is_public"`
}

// handleBucketAPI 处理桶管理API
func (s *Server) handleBucketAPI(w http.ResponseWriter, r *http.Request) {
	// 解析路径 /api/bucket/{bucket-name}/public
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[2] == "" || pathParts[3] != "public" {
		utils.WriteErrorResponse(w, "InvalidPath", "Invalid API path", http.StatusNotFound)
		return
	}

	bucketName := pathParts[2]

	if r.Method != http.MethodPut && r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	if r.Method == http.MethodPut {
		// 设置桶的公有/私有状态
		var req BucketPublicRequest
		if err := utils.ParseJSONBody(r, &req); err != nil {
			utils.WriteError(w, utils.ErrMalformedJSON, http.StatusBadRequest, "")
			return
		}

		// 检查桶是否存在
		bucket, err := s.metadata.GetBucket(bucketName)
		if err != nil {
			utils.Error("check bucket failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
		if bucket == nil {
			utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "")
			return
		}

		// 更新桶状态
		if err := s.metadata.UpdateBucketPublic(bucketName, req.IsPublic); err != nil {
			utils.Error("update bucket public status failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}

		utils.WriteJSONResponse(w, map[string]bool{"is_public": req.IsPublic})
	} else {
		// GET 获取桶的公有/私有状态
		bucket, err := s.metadata.GetBucket(bucketName)
		if err != nil {
			utils.Error("check bucket failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
			return
		}
		if bucket == nil {
			utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "")
			return
		}

		utils.WriteJSONResponse(w, map[string]bool{"is_public": bucket.IsPublic})
	}
}

// checkAuth 检查认证
func (s *Server) checkAuth(r *http.Request, w http.ResponseWriter) bool {
	hasSignature := r.URL.Query().Get("X-Amz-Signature") != ""
	hasAuthHeader := r.Header.Get("Authorization") != ""

	// 如果没有任何认证信息，拒绝访问
	if !hasSignature && !hasAuthHeader {
		utils.WriteError(w, utils.ErrAccessDenied, http.StatusForbidden, r.URL.Path)
		return false
	}

	// 验证认证信息
	if !auth.VerifyRequest(r) {
		if hasAuthHeader {
			utils.WriteError(w, utils.ErrSignatureDoesNotMatch, http.StatusForbidden, r.URL.Path)
		} else {
			utils.WriteError(w, utils.ErrAccessDenied, http.StatusForbidden, r.URL.Path)
		}
		return false
	}

	return true
}
