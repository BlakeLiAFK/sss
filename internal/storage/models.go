package storage

import "time"

// Bucket 存储桶模型
type Bucket struct {
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date"`
	IsPublic     bool      `json:"is_public"`     // 是否为公有桶
}

// Object 对象模型
type Object struct {
	Key          string    `json:"key"`
	Bucket       string    `json:"bucket"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag"`
	ContentType  string    `json:"content_type"`
	LastModified time.Time `json:"last_modified"`
	StoragePath  string    `json:"-"` // 实际存储路径
}

// MultipartUpload 多段上传模型
type MultipartUpload struct {
	UploadID    string    `json:"upload_id"`
	Bucket      string    `json:"bucket"`
	Key         string    `json:"key"`
	Initiated   time.Time `json:"initiated"`
	ContentType string    `json:"content_type"`
}

// Part 上传分片模型
type Part struct {
	UploadID   string    `json:"upload_id"`
	PartNumber int       `json:"part_number"`
	Size       int64     `json:"size"`
	ETag       string    `json:"etag"`
	ModifiedAt time.Time `json:"modified_at"`
}

// ListObjectsResult ListObjects返回结果
type ListObjectsResult struct {
	IsTruncated        bool      `xml:"IsTruncated"`
	Contents           []Object  `xml:"Contents"`
	Name               string    `xml:"Name"`
	Prefix             string    `xml:"Prefix"`
	Delimiter          string    `xml:"Delimiter"`
	MaxKeys            int       `xml:"MaxKeys"`
	CommonPrefixes     []string  `xml:"CommonPrefixes>Prefix"`
	EncodingType       string    `xml:"EncodingType,omitempty"`
	KeyCount           int       `xml:"KeyCount,omitempty"`
	ContinuationToken  string    `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string `xml:"NextContinuationToken,omitempty"`
	StartAfter         string    `xml:"StartAfter,omitempty"`
	Marker             string    `xml:"Marker,omitempty"`
	NextMarker         string    `xml:"NextMarker,omitempty"`
}
