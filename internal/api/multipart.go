package api

import (
	"encoding/xml"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"sss/internal/storage"
	"sss/internal/utils"
)

// InitiateMultipartUploadResult 初始化多段上传响应
type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

// CompleteMultipartUploadResult 完成多段上传响应
type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

// CompleteMultipartUploadRequest 完成多段上传请求
type CompleteMultipartUploadRequest struct {
	XMLName xml.Name     `xml:"CompleteMultipartUpload"`
	Parts   []PartUpload `xml:"Part"`
}

type PartUpload struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// ListPartsResult 列出分片响应
type ListPartsResult struct {
	XMLName              xml.Name   `xml:"ListPartsResult"`
	Xmlns                string     `xml:"xmlns,attr"`
	Bucket               string     `xml:"Bucket"`
	Key                  string     `xml:"Key"`
	UploadId             string     `xml:"UploadId"`
	PartNumberMarker     int        `xml:"PartNumberMarker"`
	NextPartNumberMarker int        `xml:"NextPartNumberMarker"`
	MaxParts             int        `xml:"MaxParts"`
	IsTruncated          bool       `xml:"IsTruncated"`
	Parts                []PartInfo `xml:"Part"`
}

type PartInfo struct {
	PartNumber   int    `xml:"PartNumber"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}

// handleInitiateMultipartUpload 初始化多段上传
func (s *Server) handleInitiateMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
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

	// 生成 UploadID
	uploadID := utils.GenerateID(32)

	// 获取 Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 创建多段上传记录
	upload := &storage.MultipartUpload{
		UploadID:    uploadID,
		Bucket:      bucket,
		Key:         key,
		Initiated:   time.Now().UTC(),
		ContentType: contentType,
	}

	if err := s.metadata.CreateMultipartUpload(upload); err != nil {
		utils.Error("create multipart upload failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	result := InitiateMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   bucket,
		Key:      key,
		UploadId: uploadID,
	}

	utils.WriteXML(w, http.StatusOK, result)
}

// handleUploadPart 上传分片
func (s *Server) handleUploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	// 获取分片号
	partNumberStr := r.URL.Query().Get("partNumber")
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		utils.WriteError(w, utils.ErrInvalidArgument, http.StatusBadRequest, "/"+bucket+"/"+key)
		return
	}

	// 检查多段上传是否存在
	upload, err := s.metadata.GetMultipartUpload(uploadID)
	if err != nil {
		utils.Error("get multipart upload failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}
	if upload == nil {
		utils.WriteError(w, utils.ErrNoSuchUpload, http.StatusNotFound, "/"+bucket+"/"+key)
		return
	}

	// 存储分片
	etag, size, err := s.filestore.PutPart(uploadID, partNumber, r.Body)
	if err != nil {
		utils.Error("store part failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	// 保存分片元数据
	part := &storage.Part{
		UploadID:   uploadID,
		PartNumber: partNumber,
		Size:       size,
		ETag:       etag,
		ModifiedAt: time.Now().UTC(),
	}

	if err := s.metadata.PutPart(part); err != nil {
		utils.Error("save part metadata failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	w.Header().Set("ETag", `"`+etag+`"`)
	w.WriteHeader(http.StatusOK)
}

// handleCompleteMultipartUpload 完成多段上传
func (s *Server) handleCompleteMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	// 检查多段上传是否存在
	upload, err := s.metadata.GetMultipartUpload(uploadID)
	if err != nil {
		utils.Error("get multipart upload failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}
	if upload == nil {
		utils.WriteError(w, utils.ErrNoSuchUpload, http.StatusNotFound, "/"+bucket+"/"+key)
		return
	}

	// 限制请求体大小（防止大请求攻击）
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 最大10MB

	// 解析请求体
	var completeReq CompleteMultipartUploadRequest
	decoder := xml.NewDecoder(r.Body)
	decoder.CharsetReader = nil // 使用默认字符集处理
	if err := decoder.Decode(&completeReq); err != nil {
		utils.WriteError(w, utils.ErrInvalidArgument, http.StatusBadRequest, "/"+bucket+"/"+key)
		return
	}

	// 获取已上传的分片
	dbParts, err := s.metadata.ListParts(uploadID)
	if err != nil {
		utils.Error("list parts failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	// 创建分片映射
	partMap := make(map[int]storage.Part)
	for _, p := range dbParts {
		partMap[p.PartNumber] = p
	}

	// 验证请求的分片
	var partNumbers []int
	for _, reqPart := range completeReq.Parts {
		dbPart, ok := partMap[reqPart.PartNumber]
		if !ok {
			utils.WriteError(w, utils.ErrInvalidPart, http.StatusBadRequest, "/"+bucket+"/"+key)
			return
		}
		// 验证 ETag（去掉引号）
		reqETag := strings.Trim(reqPart.ETag, `"`)
		if reqETag != dbPart.ETag {
			utils.WriteError(w, utils.ErrInvalidPart, http.StatusBadRequest, "/"+bucket+"/"+key)
			return
		}
		partNumbers = append(partNumbers, reqPart.PartNumber)
	}

	// 按分片号排序
	sort.Ints(partNumbers)

	// 合并分片
	etag, totalSize, err := s.filestore.MergeParts(bucket, key, uploadID, partNumbers)
	if err != nil {
		utils.Error("merge parts failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	// 保存对象元数据
	obj := &storage.Object{
		Key:          key,
		Bucket:       bucket,
		Size:         totalSize,
		ETag:         etag,
		ContentType:  upload.ContentType,
		LastModified: time.Now().UTC(),
		StoragePath:  s.filestore.GetStoragePath(bucket, key),
	}

	if err := s.metadata.PutObject(obj); err != nil {
		utils.Error("save object metadata failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	// 清理多段上传记录
	s.metadata.DeleteParts(uploadID)
	s.metadata.DeleteMultipartUpload(uploadID)

	result := CompleteMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: "/" + bucket + "/" + key,
		Bucket:   bucket,
		Key:      key,
		ETag:     `"` + etag + `"`,
	}

	utils.WriteXML(w, http.StatusOK, result)
}

// handleAbortMultipartUpload 取消多段上传
func (s *Server) handleAbortMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	// 检查多段上传是否存在
	upload, err := s.metadata.GetMultipartUpload(uploadID)
	if err != nil {
		utils.Error("get multipart upload failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}
	if upload == nil {
		utils.WriteError(w, utils.ErrNoSuchUpload, http.StatusNotFound, "/"+bucket+"/"+key)
		return
	}

	// 清理分片文件
	if err := s.filestore.AbortMultipartUpload(uploadID); err != nil {
		utils.Warn("abort multipart upload files failed", "error", err)
	}

	// 清理元数据
	s.metadata.DeleteParts(uploadID)
	s.metadata.DeleteMultipartUpload(uploadID)

	w.WriteHeader(http.StatusNoContent)
}

// handleListParts 列出已上传的分片
func (s *Server) handleListParts(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	// 检查多段上传是否存在
	upload, err := s.metadata.GetMultipartUpload(uploadID)
	if err != nil {
		utils.Error("get multipart upload failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}
	if upload == nil {
		utils.WriteError(w, utils.ErrNoSuchUpload, http.StatusNotFound, "/"+bucket+"/"+key)
		return
	}

	// 获取分片列表
	parts, err := s.metadata.ListParts(uploadID)
	if err != nil {
		utils.Error("list parts failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket+"/"+key)
		return
	}

	result := ListPartsResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   bucket,
		Key:      key,
		UploadId: uploadID,
		MaxParts: 1000,
	}

	for _, p := range parts {
		result.Parts = append(result.Parts, PartInfo{
			PartNumber:   p.PartNumber,
			LastModified: p.ModifiedAt.UTC().Format(time.RFC3339),
			ETag:         `"` + p.ETag + `"`,
			Size:         p.Size,
		})
	}

	if len(parts) > 0 {
		result.NextPartNumberMarker = parts[len(parts)-1].PartNumber
	}

	utils.WriteXML(w, http.StatusOK, result)
}
