package api

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sss/internal/config"
	"sss/internal/utils"
)

// ListAllMyBucketsResult ListBuckets 响应
type ListAllMyBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Xmlns   string   `xml:"xmlns,attr"`
	Owner   Owner    `xml:"Owner"`
	Buckets Buckets  `xml:"Buckets"`
}

type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type Buckets struct {
	Bucket []BucketInfo `xml:"Bucket"`
}

type BucketInfo struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

// handleListBuckets 列出所有存储桶
func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := s.metadata.ListBuckets()
	if err != nil {
		utils.Error("list buckets failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/")
		return
	}

	result := ListAllMyBucketsResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Owner: Owner{
			ID:          config.Global.Auth.AccessKeyID,
			DisplayName: "sss-user",
		},
		Buckets: Buckets{
			Bucket: make([]BucketInfo, 0, len(buckets)),
		},
	}

	for _, b := range buckets {
		result.Buckets.Bucket = append(result.Buckets.Bucket, BucketInfo{
			Name:         b.Name,
			CreationDate: b.CreationDate.UTC().Format(time.RFC3339),
		})
	}

	utils.WriteXML(w, http.StatusOK, result)
}

// handleCreateBucket 创建存储桶
func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	// 直接尝试创建，依赖数据库 PRIMARY KEY 约束处理冲突
	if err := s.metadata.CreateBucket(bucket); err != nil {
		// 检查是否是重复键错误（桶已存在）
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "PRIMARY KEY") {
			utils.WriteError(w, utils.ErrBucketAlreadyExists, http.StatusConflict, "/"+bucket)
			return
		}
		utils.Error("create bucket metadata failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
		return
	}

	// 创建目录
	if err := s.filestore.CreateBucket(bucket); err != nil {
		utils.Error("create bucket directory failed", "error", err)
		s.metadata.DeleteBucket(bucket) // 回滚
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// handleDeleteBucket 删除存储桶
func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	// 检查是否存在
	existing, err := s.metadata.GetBucket(bucket)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
		return
	}
	if existing == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "/"+bucket)
		return
	}

	// 删除元数据（会检查是否为空）
	if err := s.metadata.DeleteBucket(bucket); err != nil {
		if err.Error() == "bucket not empty" {
			utils.WriteError(w, utils.ErrBucketNotEmpty, http.StatusConflict, "/"+bucket)
		} else {
			utils.Error("delete bucket metadata failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
		}
		return
	}

	// 删除目录
	if err := s.filestore.DeleteBucket(bucket); err != nil {
		utils.Error("delete bucket directory failed", "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleHeadBucket 检查存储桶是否存在
func (s *Server) handleHeadBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	existing, err := s.metadata.GetBucket(bucket)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if existing == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("x-amz-bucket-region", config.Global.Server.Region)
	w.WriteHeader(http.StatusOK)
}

// ListBucketResult ListObjects V1 响应
type ListBucketResult struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Xmlns          string         `xml:"xmlns,attr"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix"`
	Marker         string         `xml:"Marker"`
	MaxKeys        int            `xml:"MaxKeys"`
	IsTruncated    bool           `xml:"IsTruncated"`
	Contents       []ObjectInfo   `xml:"Contents"`
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

// ListBucketResultV2 ListObjects V2 响应
type ListBucketResultV2 struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Xmlns                 string         `xml:"xmlns,attr"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	KeyCount              int            `xml:"KeyCount"`
	MaxKeys               int            `xml:"MaxKeys"`
	IsTruncated           bool           `xml:"IsTruncated"`
	Contents              []ObjectInfo   `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
	StartAfter            string         `xml:"StartAfter,omitempty"`
}

type ObjectInfo struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// handleListObjects 列出存储桶中的对象
func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	// 检查存储桶是否存在
	existing, err := s.metadata.GetBucket(bucket)
	if err != nil {
		utils.Error("check bucket failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
		return
	}
	if existing == nil {
		utils.WriteError(w, utils.ErrNoSuchBucket, http.StatusNotFound, "/"+bucket)
		return
	}

	query := r.URL.Query()
	prefix := query.Get("prefix")
	delimiter := query.Get("delimiter")
	maxKeysStr := query.Get("max-keys")
	maxKeys := 1000
	if maxKeysStr != "" {
		if n, err := strconv.Atoi(maxKeysStr); err == nil && n > 0 {
			maxKeys = n
		}
	}

	// 判断是 V1 还是 V2
	if query.Get("list-type") == "2" {
		// V2
		continuationToken := query.Get("continuation-token")
		startAfter := query.Get("start-after")
		marker := continuationToken
		if marker == "" {
			marker = startAfter
		}

		result, err := s.metadata.ListObjects(bucket, prefix, marker, delimiter, maxKeys)
		if err != nil {
			utils.Error("list objects failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
			return
		}

		response := ListBucketResultV2{
			Xmlns:             "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:              bucket,
			Prefix:            prefix,
			KeyCount:          result.KeyCount,
			MaxKeys:           maxKeys,
			IsTruncated:       result.IsTruncated,
			ContinuationToken: continuationToken,
			StartAfter:        startAfter,
		}

		if result.IsTruncated {
			response.NextContinuationToken = result.NextMarker
		}

		for _, obj := range result.Contents {
			response.Contents = append(response.Contents, ObjectInfo{
				Key:          obj.Key,
				LastModified: obj.LastModified.UTC().Format(time.RFC3339),
				ETag:         `"` + obj.ETag + `"`,
				Size:         obj.Size,
				StorageClass: "STANDARD",
			})
		}

		for _, p := range result.CommonPrefixes {
			response.CommonPrefixes = append(response.CommonPrefixes, CommonPrefix{Prefix: p})
		}

		utils.WriteXML(w, http.StatusOK, response)
	} else {
		// V1
		marker := query.Get("marker")

		result, err := s.metadata.ListObjects(bucket, prefix, marker, delimiter, maxKeys)
		if err != nil {
			utils.Error("list objects failed", "error", err)
			utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "/"+bucket)
			return
		}

		response := ListBucketResult{
			Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:        bucket,
			Prefix:      prefix,
			Marker:      marker,
			MaxKeys:     maxKeys,
			IsTruncated: result.IsTruncated,
		}

		for _, obj := range result.Contents {
			response.Contents = append(response.Contents, ObjectInfo{
				Key:          obj.Key,
				LastModified: obj.LastModified.UTC().Format(time.RFC3339),
				ETag:         `"` + obj.ETag + `"`,
				Size:         obj.Size,
				StorageClass: "STANDARD",
			})
		}

		for _, p := range result.CommonPrefixes {
			response.CommonPrefixes = append(response.CommonPrefixes, CommonPrefix{Prefix: p})
		}

		utils.WriteXML(w, http.StatusOK, response)
	}
}
