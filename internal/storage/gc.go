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
	OrphanFiles     []OrphanFile `json:"orphan_files"`      // 孤立文件列表
	OrphanCount     int          `json:"orphan_count"`      // 孤立文件数量
	OrphanSize      int64        `json:"orphan_size"`       // 孤立文件总大小
	ExpiredUploads  []string     `json:"expired_uploads"`   // 过期的分片上传ID
	ExpiredCount    int          `json:"expired_count"`     // 过期上传数量
	ExpiredPartSize int64        `json:"expired_part_size"` // 过期分片总大小
	Cleaned         bool         `json:"cleaned"`           // 是否已清理
	CleanedAt       *time.Time   `json:"cleaned_at"`        // 清理时间
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

	// 获取所有元数据中的存储路径
	knownPaths := make(map[string]bool)
	buckets, err := metadata.ListBuckets()
	if err != nil {
		return nil, err
	}

	for _, bucket := range buckets {
		// 获取桶中所有对象
		objects, err := metadata.ListAllObjects(bucket.Name)
		if err != nil {
			return nil, err
		}
		for _, obj := range objects {
			knownPaths[obj.StoragePath] = true
		}
	}

	// 遍历磁盘文件
	err = filepath.Walk(f.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误继续
		}

		// 跳过目录
		if info.IsDir() {
			// 跳过 .multipart 目录（由 ScanExpiredUploads 处理）
			if info.Name() == ".multipart" {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查文件是否在元数据中
		if !knownPaths[path] {
			relPath, _ := filepath.Rel(f.basePath, path)
			result.OrphanFiles = append(result.OrphanFiles, OrphanFile{
				Path:       relPath,
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
			})
			result.OrphanSize += info.Size()
		}

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
		if !strings.HasPrefix(cleanPath, f.basePath) {
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
		m.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM parts WHERE upload_id = ?", uploadID).Scan(&partSize)
		totalCleaned += partSize

		// 删除分片记录
		if _, err := m.db.Exec("DELETE FROM parts WHERE upload_id = ?", uploadID); err != nil {
			return totalCleaned, err
		}

		// 删除上传记录
		if _, err := m.db.Exec("DELETE FROM multipart_uploads WHERE upload_id = ?", uploadID); err != nil {
			return totalCleaned, err
		}

		// 删除磁盘上的分片文件
		if filestore != nil {
			filestore.AbortMultipartUpload(uploadID)
		}
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
	multipartDir := filepath.Join(f.basePath, ".multipart")

	// 检查目录是否存在
	if _, err := os.Stat(multipartDir); os.IsNotExist(err) {
		return nil, 0, nil
	}

	// 获取所有活跃的上传ID
	activeUploads := make(map[string]bool)
	rows, err := metadata.db.Query("SELECT upload_id FROM multipart_uploads")
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var uploadID string
		if err := rows.Scan(&uploadID); err != nil {
			return nil, 0, err
		}
		activeUploads[uploadID] = true
	}

	var orphans []OrphanFile
	var totalSize int64

	// 遍历 .multipart 目录
	entries, err := os.ReadDir(multipartDir)
	if err != nil {
		return nil, 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		uploadID := entry.Name()
		if activeUploads[uploadID] {
			continue // 活跃上传，跳过
		}

		// 计算孤立上传目录的大小
		uploadDir := filepath.Join(multipartDir, uploadID)
		err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(f.basePath, path)
			orphans = append(orphans, OrphanFile{
				Path:       relPath,
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
			})
			totalSize += info.Size()
			return nil
		})
		if err != nil {
			continue
		}
	}

	return orphans, totalSize, nil
}

// RunGC 执行完整的垃圾回收
func RunGC(filestore *FileStore, metadata *MetadataStore, maxUploadAge time.Duration, dryRun bool) (*GCResult, error) {
	result := &GCResult{
		OrphanFiles:    make([]OrphanFile, 0),
		ExpiredUploads: make([]string, 0),
	}

	// 1. 扫描孤立文件
	orphanResult, err := filestore.ScanOrphanFiles(metadata)
	if err != nil {
		return nil, err
	}
	result.OrphanFiles = orphanResult.OrphanFiles
	result.OrphanCount = orphanResult.OrphanCount
	result.OrphanSize = orphanResult.OrphanSize

	// 2. 扫描 .multipart 中的孤立分片
	multipartOrphans, multipartSize, err := filestore.ScanMultipartOrphans(metadata)
	if err == nil && len(multipartOrphans) > 0 {
		result.OrphanFiles = append(result.OrphanFiles, multipartOrphans...)
		result.OrphanCount += len(multipartOrphans)
		result.OrphanSize += multipartSize
	}

	// 3. 扫描过期上传
	expiredUploads, err := metadata.GetExpiredUploads(maxUploadAge)
	if err != nil {
		return nil, err
	}
	for _, u := range expiredUploads {
		result.ExpiredUploads = append(result.ExpiredUploads, u.UploadID)
		result.ExpiredPartSize += u.TotalSize
	}
	result.ExpiredCount = len(expiredUploads)

	// 如果不是干运行模式，执行清理
	if !dryRun {
		// 清理孤立文件
		if len(result.OrphanFiles) > 0 {
			if err := filestore.CleanOrphanFiles(result.OrphanFiles); err != nil {
				return result, err
			}
		}

		// 清理过期上传
		if len(result.ExpiredUploads) > 0 {
			if _, err := metadata.CleanExpiredUploads(result.ExpiredUploads, filestore); err != nil {
				return result, err
			}
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
