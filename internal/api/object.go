package api

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// handleGetObject 获取对象
func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	// 检查存储桶
	b, err := s.metadata.GetBucket(bucket)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}
	if b == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "/"+bucket)
		return
	}

	// 获取对象元数据
	obj, err := s.metadata.GetObject(bucket, key)
	if err != nil {
		utils.Error("get object metadata failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}
	if obj == nil {
		utils.WriteError(w, utils.ErrNoSuchKey, http.StatusNotFound, "/"+bucket+"/"+key)
		return
	}

	// 打开文件
	file, err := s.filestore.GetObject(obj.StoragePath)
	if err != nil {
		utils.Error("get object file failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}
	defer file.Close()

	// 处理 Range 请求
	var start, end int64 = 0, obj.Size - 1
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		if strings.HasPrefix(rangeHeader, "bytes=") {
			rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.Split(rangeSpec, "-")
			if len(parts) == 2 {
				if parts[0] != "" {
					start, _ = strconv.ParseInt(parts[0], 10, 64)
				}
				if parts[1] != "" {
					end, _ = strconv.ParseInt(parts[1], 10, 64)
				} else if parts[0] != "" {
					end = obj.Size - 1
				}
			}
		}
	}

	// 设置响应头
	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("ETag", `"`+obj.ETag+`"`)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")

	if rangeHeader != "" && start > 0 {
		w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(obj.Size, 10))
		w.WriteHeader(http.StatusPartialContent)
		file.Seek(start, 0)
		io.CopyN(w, file, end-start+1)
	} else {
		w.WriteHeader(http.StatusOK)
		io.Copy(w, file)
	}
}

// handlePutObject 上传对象
func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	// 检查存储桶
	b, err := s.metadata.GetBucket(bucket)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
		return
	}
	if b == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "/"+bucket)
		return
	}

	// 验证文件大小限制
	query := r.URL.Query()

	// 1. 检查预签名URL的大小限制（如果有）
	if maxContentLengthStr := query.Get("X-Amz-Max-Content-Length"); maxContentLengthStr != "" {
		maxContentLength, err := strconv.ParseInt(maxContentLengthStr, 10, 64)
		if err == nil {
			if r.ContentLength > 0 && r.ContentLength > maxContentLength {
				utils.WriteError(w, utils.ErrEntityTooLarge, http.StatusBadRequest, "/"+bucket+"/"+key)
				return
			}
		}
	}

	// 2. 检查全局最大上传大小限制
	if config.Global.Storage.MaxUploadSize > 0 && r.ContentLength > 0 {
		if r.ContentLength > config.Global.Storage.MaxUploadSize {
			utils.WriteError(w, utils.ErrEntityTooLarge, http.StatusBadRequest, "/"+bucket+"/"+key)
			return
		}
	}

	// 3. 检查全局最大对象大小限制
	if config.Global.Storage.MaxObjectSize > 0 && r.ContentLength > 0 {
		if r.ContentLength > config.Global.Storage.MaxObjectSize {
			utils.WriteError(w, utils.ErrEntityTooLarge, http.StatusBadRequest, "/"+bucket+"/"+key)
			return
		}
	}

	// 获取 Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 4. 验证内容类型限制（如果预签名URL指定了）
	if expectedContentType := query.Get("X-Amz-Content-Type"); expectedContentType != "" {
		if contentType != expectedContentType {
			utils.WriteError(w, utils.ErrBadDigest, http.StatusBadRequest, "/"+bucket+"/"+key)
			return
		}
	}

	// 存储文件
	storagePath, etag, err := s.filestore.PutObject(bucket, key, r.Body, r.ContentLength)
	if err != nil {
		utils.Error("store object failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	// 保存元数据
	obj := &storage.Object{
		Key:          key,
		Bucket:       bucket,
		Size:         r.ContentLength,
		ETag:         etag,
		ContentType:  contentType,
		LastModified: time.Now().UTC(),
		StoragePath:  storagePath,
	}

	if err := s.metadata.PutObject(obj); err != nil {
		utils.Error("save object metadata failed", "error", err)
		s.filestore.DeleteObject(storagePath) // 回滚
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	w.Header().Set("ETag", `"`+etag+`"`)
	w.WriteHeader(http.StatusOK)
}

// handleDeleteObject 删除对象
func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	// 获取对象元数据
	obj, err := s.metadata.GetObject(bucket, key)
	if err != nil {
		utils.Error("get object metadata failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	if obj != nil {
		// 删除文件
		if err := s.filestore.DeleteObject(obj.StoragePath); err != nil {
			utils.Warn("delete object file failed", "error", err)
		}

		// 删除元数据
		if err := s.metadata.DeleteObject(bucket, key); err != nil {
			utils.Error("delete object metadata failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
			return
		}
	}

	// S3 删除不存在的对象也返回 204
	w.WriteHeader(http.StatusNoContent)
}

// handleHeadObject 获取对象元数据
func (s *Server) handleHeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	// 检查存储桶
	b, err := s.metadata.GetBucket(bucket)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if b == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// 获取对象元数据
	obj, err := s.metadata.GetObject(bucket, key)
	if err != nil {
		utils.Error("get object metadata failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if obj == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	w.Header().Set("ETag", `"`+obj.ETag+`"`)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
}
