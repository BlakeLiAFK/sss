package api

import (
	"io"
	"net/http"
	"net/url"
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
	if rangeHeader != "" && obj.Size > 0 {
		if strings.HasPrefix(rangeHeader, "bytes=") {
			rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.Split(rangeSpec, "-")
			if len(parts) == 2 {
				if parts[0] != "" {
					parsedStart, err := strconv.ParseInt(parts[0], 10, 64)
					if err == nil && parsedStart >= 0 {
						start = parsedStart
					}
				}
				if parts[1] != "" {
					parsedEnd, err := strconv.ParseInt(parts[1], 10, 64)
					if err == nil && parsedEnd >= 0 {
						end = parsedEnd
					}
				} else if parts[0] != "" {
					end = obj.Size - 1
				}
			}
		}
		// 验证范围有效性
		if start < 0 {
			start = 0
		}
		if end >= obj.Size {
			end = obj.Size - 1
		}
		if start > end {
			// 无效范围，返回416
			w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(obj.Size, 10))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	// 设置响应头
	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("ETag", `"`+obj.ETag+`"`)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")

	if rangeHeader != "" {
		// Range 请求：返回 206 Partial Content
		w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(obj.Size, 10))
		w.WriteHeader(http.StatusPartialContent)
		if start > 0 {
			if _, err := file.Seek(start, 0); err != nil {
				utils.Error("seek file failed", "error", err)
				return
			}
		}
		if _, err := io.CopyN(w, file, end-start+1); err != nil {
			// 客户端可能已断开连接，只记录日志
			utils.Debug("copy to response failed", "error", err)
		}
	} else {
		// 普通请求：返回 200 OK
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, file); err != nil {
			// 客户端可能已断开连接，只记录日志
			utils.Debug("copy to response failed", "error", err)
		}
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

// handleCopyObject 复制对象
func (s *Server) handleCopyObject(w http.ResponseWriter, r *http.Request, destBucket, destKey string) {
	// 解析源对象路径
	copySource := r.Header.Get("x-amz-copy-source")
	if copySource == "" {
		utils.WriteError(w, utils.ErrInvalidArgument, http.StatusBadRequest, "/"+destBucket+"/"+destKey)
		return
	}

	// URL解码源路径（处理中文文件名等）
	decodedSource, err := url.PathUnescape(copySource)
	if err != nil {
		utils.WriteErrorResponse(w, "InvalidCopySource", "Invalid x-amz-copy-source encoding", http.StatusBadRequest)
		return
	}

	// 解析源路径，格式: /bucket/key 或 bucket/key
	decodedSource = strings.TrimPrefix(decodedSource, "/")
	parts := strings.SplitN(decodedSource, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		utils.WriteErrorResponse(w, "InvalidCopySource", "Invalid x-amz-copy-source format", http.StatusBadRequest)
		return
	}
	srcBucket := parts[0]
	srcKey := parts[1]

	// 验证路径安全性（防止路径遍历）
	if strings.Contains(srcBucket, "..") || strings.ContainsAny(srcBucket, "/\\") {
		utils.WriteErrorResponse(w, "InvalidCopySource", "Invalid source bucket name", http.StatusBadRequest)
		return
	}
	if strings.Contains(srcKey, "..") {
		utils.WriteErrorResponse(w, "InvalidCopySource", "Invalid source key", http.StatusBadRequest)
		return
	}

	// 检查源存储桶
	srcB, err := s.metadata.GetBucket(srcBucket)
	if err != nil {
		utils.Error("check source bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+srcBucket)
		return
	}
	if srcB == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "/"+srcBucket)
		return
	}

	// 检查目标存储桶
	destB, err := s.metadata.GetBucket(destBucket)
	if err != nil {
		utils.Error("check dest bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+destBucket)
		return
	}
	if destB == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "/"+destBucket)
		return
	}

	// 获取源对象元数据
	srcObj, err := s.metadata.GetObject(srcBucket, srcKey)
	if err != nil {
		utils.Error("get source object metadata failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+srcBucket+"/"+srcKey)
		return
	}
	if srcObj == nil {
		utils.WriteError(w, utils.ErrNoSuchKey, http.StatusNotFound, "/"+srcBucket+"/"+srcKey)
		return
	}

	// 复制文件
	newStoragePath, etag, err := s.filestore.CopyObject(srcObj.StoragePath, destBucket, destKey)
	if err != nil {
		utils.Error("copy object file failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+destBucket+"/"+destKey)
		return
	}

	// 保存新对象元数据
	newObj := &storage.Object{
		Key:          destKey,
		Bucket:       destBucket,
		Size:         srcObj.Size,
		ETag:         etag,
		ContentType:  srcObj.ContentType,
		LastModified: time.Now().UTC(),
		StoragePath:  newStoragePath,
	}

	if err := s.metadata.PutObject(newObj); err != nil {
		utils.Error("save copied object metadata failed", "error", err)
		s.filestore.DeleteObject(newStoragePath) // 回滚
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+destBucket+"/"+destKey)
		return
	}

	// 返回 S3 CopyObject 响应格式
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	response := `<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <LastModified>` + newObj.LastModified.Format(time.RFC3339) + `</LastModified>
  <ETag>"` + etag + `"</ETag>
</CopyObjectResult>`
	w.Write([]byte(response))
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
