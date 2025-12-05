package api

import (
	"net/http"
	"strings"

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
	// 认证检查（预签名URL或Authorization头）
	if r.URL.Query().Get("X-Amz-Signature") == "" && r.Header.Get("Authorization") != "" {
		if !auth.VerifyRequest(r) {
			utils.WriteError(w, utils.ErrSignatureDoesNotMatch, http.StatusForbidden, r.URL.Path)
			return
		}
	} else if r.URL.Query().Get("X-Amz-Signature") != "" {
		if !auth.VerifyRequest(r) {
			utils.WriteError(w, utils.ErrAccessDenied, http.StatusForbidden, r.URL.Path)
			return
		}
	}

	// 解析路径
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)

	bucket := ""
	key := ""
	if len(parts) >= 1 {
		bucket = parts[0]
	}
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
