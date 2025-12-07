package storage

import (
	"os"
	"path/filepath"
	"strings"
)

// StorageStats 存储统计信息
type StorageStats struct {
	TotalBuckets int          `json:"total_buckets"` // 桶总数
	TotalObjects int          `json:"total_objects"` // 对象总数
	TotalSize    int64        `json:"total_size"`    // 总大小(字节)
	BucketStats  []BucketStat `json:"bucket_stats"`  // 各桶统计
	TypeStats    []TypeStat   `json:"type_stats"`    // 文件类型统计
}

// BucketStat 单个桶的统计
type BucketStat struct {
	Name        string `json:"name"`
	ObjectCount int    `json:"object_count"`
	TotalSize   int64  `json:"total_size"`
	IsPublic    bool   `json:"is_public"`
}

// TypeStat 文件类型统计
type TypeStat struct {
	ContentType string `json:"content_type"` // MIME 类型
	Extension   string `json:"extension"`    // 文件扩展名
	Count       int    `json:"count"`
	TotalSize   int64  `json:"total_size"`
}

// GetStorageStats 获取存储统计信息
func (m *MetadataStore) GetStorageStats() (*StorageStats, error) {
	stats := &StorageStats{}

	// 1. 获取桶总数
	err := m.db.QueryRow("SELECT COUNT(*) FROM buckets").Scan(&stats.TotalBuckets)
	if err != nil {
		return nil, err
	}

	// 2. 获取对象总数和总大小
	err = m.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(size), 0) FROM objects").
		Scan(&stats.TotalObjects, &stats.TotalSize)
	if err != nil {
		return nil, err
	}

	// 3. 获取各桶统计
	rows, err := m.db.Query(`
		SELECT b.name, b.is_public,
			   COUNT(o.key) as object_count,
			   COALESCE(SUM(o.size), 0) as total_size
		FROM buckets b
		LEFT JOIN objects o ON b.name = o.bucket
		GROUP BY b.name, b.is_public
		ORDER BY total_size DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var bs BucketStat
		if err := rows.Scan(&bs.Name, &bs.IsPublic, &bs.ObjectCount, &bs.TotalSize); err != nil {
			return nil, err
		}
		stats.BucketStats = append(stats.BucketStats, bs)
	}

	// 4. 获取文件类型统计
	typeRows, err := m.db.Query(`
		SELECT content_type, COUNT(*) as count, SUM(size) as total_size
		FROM objects
		WHERE content_type IS NOT NULL AND content_type != ''
		GROUP BY content_type
		ORDER BY total_size DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var ts TypeStat
		if err := typeRows.Scan(&ts.ContentType, &ts.Count, &ts.TotalSize); err != nil {
			return nil, err
		}
		// 从 content_type 提取扩展名显示
		ts.Extension = getExtensionFromContentType(ts.ContentType)
		stats.TypeStats = append(stats.TypeStats, ts)
	}

	return stats, nil
}

// getExtensionFromContentType 从 MIME 类型获取友好扩展名
func getExtensionFromContentType(contentType string) string {
	// 常见 MIME 类型映射
	mimeToExt := map[string]string{
		"image/png":                "PNG",
		"image/jpeg":               "JPEG",
		"image/gif":                "GIF",
		"image/webp":               "WebP",
		"image/svg+xml":            "SVG",
		"text/plain":               "TXT",
		"text/html":                "HTML",
		"text/css":                 "CSS",
		"text/javascript":          "JS",
		"application/json":         "JSON",
		"application/xml":          "XML",
		"application/pdf":          "PDF",
		"application/zip":          "ZIP",
		"application/gzip":         "GZIP",
		"application/x-tar":        "TAR",
		"video/mp4":                "MP4",
		"video/webm":               "WebM",
		"audio/mpeg":               "MP3",
		"audio/wav":                "WAV",
		"application/octet-stream": "Binary",
	}

	if ext, ok := mimeToExt[contentType]; ok {
		return ext
	}

	// 尝试从 MIME 类型提取
	parts := strings.Split(contentType, "/")
	if len(parts) == 2 {
		return strings.ToUpper(parts[1])
	}

	return "Other"
}

// GetRecentObjects 获取最近上传的对象
func (m *MetadataStore) GetRecentObjects(limit int) ([]Object, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := m.db.Query(`
		SELECT bucket, key, size, etag, content_type, last_modified, storage_path
		FROM objects
		ORDER BY last_modified DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objects []Object
	for rows.Next() {
		var obj Object
		if err := rows.Scan(&obj.Bucket, &obj.Key, &obj.Size, &obj.ETag,
			&obj.ContentType, &obj.LastModified, &obj.StoragePath); err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// GetDiskUsage 获取磁盘使用统计（实际文件系统）
func (f *FileStore) GetDiskUsage() (totalSize int64, fileCount int, err error) {
	err = filepath.Walk(f.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误继续
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})
	return
}
