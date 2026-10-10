package storage

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GCResult 垃圾回收结果
type GCResult struct {
	OrphanFiles      []OrphanFile `json:"orphan_files"`      // 孤立文件列表
	OrphanCount      int          `json:"orphan_count"`      // 孤立文件数量
	OrphanSize       int64        `json:"orphan_size"`       // 孤立文件总大小
	ExpiredUploads   []string     `json:"expired_uploads"`   // 过期的分片上传ID
	ExpiredCount     int          `json:"expired_count"`     // 过期上传数量
	ExpiredPartSize  int64        `json:"expired_part_size"` // 过期分片总大小
	DetailsTruncated bool         `json:"details_truncated"` // Detail arrays are samples; counts remain complete.
	Cleaned          bool         `json:"cleaned"`           // 是否已清理
	CleanedAt        *time.Time   `json:"cleaned_at"`        // 清理时间
}

// OrphanFile 孤立文件信息
type OrphanFile struct {
	Path       string    `json:"path"`        // 相对路径
	Size       int64     `json:"size"`        // 文件大小
	ModifiedAt time.Time `json:"modified_at"` // 修改时间
}

// ExpiredUploadInfo 过期上传信息
type ExpiredUploadInfo struct {
	UploadID    string    `json:"upload_id"`
	Bucket      string    `json:"bucket"`
	Key         string    `json:"key"`
	Initiated   time.Time `json:"initiated"`
	ContentType string    `json:"content_type"`
	PartCount   int       `json:"part_count"`
	TotalSize   int64     `json:"total_size"`
}

// ScanOrphanFiles 扫描孤立文件（元数据中不存在但磁盘上存在的文件）
func (f *FileStore) ScanOrphanFiles(metadata *MetadataStore) (*GCResult, error) {
	result := &GCResult{
		OrphanFiles:    make([]OrphanFile, 0),
		ExpiredUploads: make([]string, 0),
	}

	err := f.scanObjectOrphans(metadata, func(file OrphanFile) error {
		result.OrphanFiles = append(result.OrphanFiles, file)
		result.OrphanSize += file.Size
		return nil
	})
	if err != nil {
		return nil, err
	}

	result.OrphanCount = len(result.OrphanFiles)
	return result, nil
}

// CleanOrphanFiles 清理孤立文件
func (f *FileStore) CleanOrphanFiles(files []OrphanFile) error {
	for _, file := range files {
		fullPath := filepath.Join(f.basePath, file.Path)

		// 安全检查：确保路径在 basePath 下
		cleanPath := filepath.Clean(fullPath)
		if !withinBase(f.basePath, cleanPath) {
			continue // 跳过可疑路径
		}

		if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
			return err
		}

		// 尝试清理空目录
		dir := filepath.Dir(cleanPath)
		f.cleanEmptyDirs(dir)
	}
	return nil
}

// cleanEmptyDirs 递归清理空目录，直到 basePath
func (f *FileStore) cleanEmptyDirs(dir string) {
	for strings.HasPrefix(dir, f.basePath) && dir != f.basePath {
		// 检查目录是否为空
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}

		// 删除空目录
		if err := os.Remove(dir); err != nil {
			return
		}

		// 继续检查父目录
		dir = filepath.Dir(dir)
	}
}

// GetExpiredUploads 获取过期的分片上传
func (m *MetadataStore) GetExpiredUploads(maxAge time.Duration) ([]ExpiredUploadInfo, error) {
	cutoff := time.Now().Add(-maxAge)

	rows, err := m.db.Query(`
		SELECT mu.upload_id, mu.bucket, mu.key, mu.initiated, mu.content_type,
		       COUNT(p.part_number) as part_count,
		       COALESCE(SUM(p.size), 0) as total_size
		FROM multipart_uploads mu
		LEFT JOIN parts p ON mu.upload_id = p.upload_id
		WHERE mu.initiated < ?
		GROUP BY mu.upload_id
		ORDER BY mu.initiated
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var uploads []ExpiredUploadInfo
	for rows.Next() {
		var u ExpiredUploadInfo
		if err := rows.Scan(&u.UploadID, &u.Bucket, &u.Key, &u.Initiated,
			&u.ContentType, &u.PartCount, &u.TotalSize); err != nil {
			return nil, err
		}
		uploads = append(uploads, u)
	}

	return uploads, nil
}

// CleanExpiredUploads 清理过期的分片上传
func (m *MetadataStore) CleanExpiredUploads(uploadIDs []string, filestore *FileStore) (int64, error) {
	var totalCleaned int64

	for _, uploadID := range uploadIDs {
		// 计算分片大小
		var partSize int64
		if err := m.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM parts WHERE upload_id = ?", uploadID).Scan(&partSize); err != nil {
			return totalCleaned, err
		}
		if err := m.AbortMultipart(uploadID); err != nil {
			return totalCleaned, err
		}
		if filestore != nil {
			if err := filestore.AbortMultipartUpload(uploadID); err != nil {
				return totalCleaned, err
			}
		}
		totalCleaned += partSize

	}

	return totalCleaned, nil
}

// ListAllObjects 列出桶中所有对象（无分页限制，内部使用）
func (m *MetadataStore) ListAllObjects(bucket string) ([]Object, error) {
	rows, err := m.db.Query(`
		SELECT bucket, key, size, etag, content_type, last_modified, storage_path
		FROM objects
		WHERE bucket = ?
		ORDER BY key
	`, bucket)
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

// ScanMultipartOrphans 扫描 .multipart 目录中的孤立分片
func (f *FileStore) ScanMultipartOrphans(metadata *MetadataStore) ([]OrphanFile, int64, error) {
	var orphans []OrphanFile
	var size int64
	err := f.scanMultipartOrphans(metadata, func(file OrphanFile) error { orphans = append(orphans, file); size += file.Size; return nil })
	return orphans, size, err
}

// RunGC 执行完整的垃圾回收
// The caller holds the exclusive maintenance gate for both preview and delete.
// Counts cover the whole store; response details are capped samples. Deletion
// streams from the scanner, never from the sample arrays.
func RunGC(filestore *FileStore, metadata *MetadataStore, maxUploadAge time.Duration, dryRun bool) (*GCResult, error) {
	result := &GCResult{OrphanFiles: make([]OrphanFile, 0), ExpiredUploads: make([]string, 0)}
	visit := func(file OrphanFile) error {
		result.OrphanCount++
		result.OrphanSize += file.Size
		if len(result.OrphanFiles) < GCDetailLimit {
			result.OrphanFiles = append(result.OrphanFiles, file)
		} else {
			result.DetailsTruncated = true
		}
		if !dryRun {
			path := filepath.Join(filestore.basePath, file.Path)
			if !withinBase(filestore.basePath, path) {
				return ErrInvalidPath
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	if err := filestore.scanObjectOrphans(metadata, visit); err != nil {
		return result, err
	}
	if err := filestore.scanMultipartOrphans(metadata, visit); err != nil {
		return result, err
	}
	cutoff := time.Now().Add(-maxUploadAge)
	marker := ""
	for {
		rows, err := metadata.db.Query("SELECT upload_id FROM multipart_uploads WHERE initiated < ? AND upload_id > ? ORDER BY upload_id LIMIT ?", cutoff, marker, gcBatchSize)
		if err != nil {
			return result, err
		}
		ids := make([]string, 0, gcBatchSize)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return result, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			var size int64
			if err := metadata.db.QueryRow("SELECT COALESCE(SUM(size),0) FROM parts WHERE upload_id=?", id).Scan(&size); err != nil {
				return result, err
			}
			result.ExpiredCount++
			result.ExpiredPartSize += size
			if len(result.ExpiredUploads) < GCDetailLimit {
				result.ExpiredUploads = append(result.ExpiredUploads, id)
			} else {
				result.DetailsTruncated = true
			}
			if !dryRun {
				if _, err := metadata.CleanExpiredUploads([]string{id}, filestore); err != nil {
					return result, err
				}
			}
		}
		marker = ids[len(ids)-1]
	}
	if !dryRun {
		if _, err := pruneEmptyGCDirectories(filestore.basePath, false); err != nil {
			return result, err
		}
		result.Cleaned = true
		now := time.Now()
		result.CleanedAt = &now
	}
	return result, nil
}

// GetStoragePathFromKey 根据 bucket 和 key 计算预期的存储路径
func (f *FileStore) GetStoragePathFromKey(bucket, key string) string {
	h := md5.Sum([]byte(key))
	subdir := hex.EncodeToString(h[:1])
	return filepath.Join(f.basePath, bucket, subdir, key)
}
