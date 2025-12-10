package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// MetadataStore SQLite元数据存储
type MetadataStore struct {
	db    *sql.DB
	wmu   sync.Mutex // 写操作互斥锁，确保写入串行化
}

// NewMetadataStore 创建元数据存储
func NewMetadataStore(dbPath string) (*MetadataStore, error) {
	// modernc.org/sqlite 使用不同的参数格式
	// 使用 WAL 模式提升并发性能，设置 busy_timeout 避免锁等待
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(2000)")
	if err != nil {
		return nil, err
	}

	// 设置连接池参数（SQLite + WAL 模式优化）
	// WAL 模式允许：多个读取者 + 一个写入者 同时工作
	// 写操作通过应用层 wmu 互斥锁串行化
	db.SetMaxOpenConns(10)   // 允许多个并发读取
	db.SetMaxIdleConns(5)    // 保持空闲连接
	db.SetConnMaxLifetime(0) // 连接不过期
	db.SetConnMaxIdleTime(0) // 空闲连接不过期

	// 验证连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &MetadataStore{db: db}
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initTables 初始化数据库表
func (m *MetadataStore) initTables() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS buckets (
			name TEXT PRIMARY KEY,
			creation_date DATETIME NOT NULL,
			is_public INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS objects (
			bucket TEXT NOT NULL,
			key TEXT NOT NULL,
			size INTEGER NOT NULL,
			etag TEXT NOT NULL,
			content_type TEXT,
			last_modified DATETIME NOT NULL,
			storage_path TEXT NOT NULL,
			PRIMARY KEY (bucket, key),
			FOREIGN KEY (bucket) REFERENCES buckets(name) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS multipart_uploads (
			upload_id TEXT PRIMARY KEY,
			bucket TEXT NOT NULL,
			key TEXT NOT NULL,
			initiated DATETIME NOT NULL,
			content_type TEXT,
			FOREIGN KEY (bucket) REFERENCES buckets(name) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS parts (
			upload_id TEXT NOT NULL,
			part_number INTEGER NOT NULL,
			size INTEGER NOT NULL,
			etag TEXT NOT NULL,
			modified_at DATETIME NOT NULL,
			PRIMARY KEY (upload_id, part_number),
			FOREIGN KEY (upload_id) REFERENCES multipart_uploads(upload_id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_objects_bucket ON objects(bucket)`,
		`CREATE INDEX IF NOT EXISTS idx_objects_prefix ON objects(bucket, key)`,
		// API Keys 表
		`CREATE TABLE IF NOT EXISTS api_keys (
			access_key_id TEXT PRIMARY KEY,
			secret_access_key TEXT NOT NULL,
			description TEXT,
			created_at DATETIME NOT NULL,
			enabled INTEGER DEFAULT 1
		)`,
		// API Key 桶权限表
		`CREATE TABLE IF NOT EXISTS api_key_permissions (
			access_key_id TEXT NOT NULL,
			bucket_name TEXT NOT NULL,
			can_read INTEGER DEFAULT 0,
			can_write INTEGER DEFAULT 0,
			PRIMARY KEY (access_key_id, bucket_name),
			FOREIGN KEY (access_key_id) REFERENCES api_keys(access_key_id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_permissions ON api_key_permissions(access_key_id)`,
		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
	}

	for _, schema := range schemas {
		if _, err := m.db.Exec(schema); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// 检查并添加is_public列（用于兼容现有数据）
	var columnExists bool
	err := m.db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('buckets')
		WHERE name = 'is_public'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("check column failed: %v", err)
	}

	if !columnExists {
		if _, err := m.db.Exec("ALTER TABLE buckets ADD COLUMN is_public INTEGER DEFAULT 0"); err != nil {
			return fmt.Errorf("add is_public column failed: %v", err)
		}
	}

	// 初始化审计日志表
	if err := m.initAuditTable(); err != nil {
		return fmt.Errorf("init audit table failed: %v", err)
	}

	// 初始化 GeoStats 表
	if err := m.initGeoStatsTable(); err != nil {
		return fmt.Errorf("init geo_stats table failed: %v", err)
	}

	return nil
}

// Close 关闭数据库连接
func (m *MetadataStore) Close() error {
	return m.db.Close()
}

// withWriteLock 执行写操作（带互斥锁）
func (m *MetadataStore) withWriteLock(fn func() error) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	return fn()
}

// === Bucket 操作 ===

func (m *MetadataStore) CreateBucket(name string) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec(
			"INSERT INTO buckets (name, creation_date, is_public) VALUES (?, ?, ?)",
			name, time.Now().UTC(), 0,
		)
		return err
	})
}

func (m *MetadataStore) DeleteBucket(name string) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()

	// 使用事务确保检查和删除的原子性
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 检查是否有对象
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM objects WHERE bucket = ?", name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("bucket not empty")
	}

	// 删除桶
	if _, err := tx.Exec("DELETE FROM buckets WHERE name = ?", name); err != nil {
		return err
	}

	return tx.Commit()
}

func (m *MetadataStore) GetBucket(name string) (*Bucket, error) {
	var bucket Bucket
	err := m.db.QueryRow(
		"SELECT name, creation_date, is_public FROM buckets WHERE name = ?", name,
	).Scan(&bucket.Name, &bucket.CreationDate, &bucket.IsPublic)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &bucket, err
}

func (m *MetadataStore) ListBuckets() ([]Bucket, error) {
	rows, err := m.db.Query("SELECT name, creation_date, is_public FROM buckets ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []Bucket
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Name, &b.CreationDate, &b.IsPublic); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, nil
}

// UpdateBucketPublic 设置桶的公有/私有状态
func (m *MetadataStore) UpdateBucketPublic(name string, isPublic bool) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec(
			"UPDATE buckets SET is_public = ? WHERE name = ?",
			isPublic, name,
		)
		return err
	})
}

// === Object 操作 ===

func (m *MetadataStore) PutObject(obj *Object) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec(`
			INSERT OR REPLACE INTO objects (bucket, key, size, etag, content_type, last_modified, storage_path)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			obj.Bucket, obj.Key, obj.Size, obj.ETag, obj.ContentType, obj.LastModified, obj.StoragePath,
		)
		return err
	})
}

func (m *MetadataStore) GetObject(bucket, key string) (*Object, error) {
	var obj Object
	err := m.db.QueryRow(`
		SELECT bucket, key, size, etag, content_type, last_modified, storage_path
		FROM objects WHERE bucket = ? AND key = ?`,
		bucket, key,
	).Scan(&obj.Bucket, &obj.Key, &obj.Size, &obj.ETag, &obj.ContentType, &obj.LastModified, &obj.StoragePath)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &obj, err
}

func (m *MetadataStore) DeleteObject(bucket, key string) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec("DELETE FROM objects WHERE bucket = ? AND key = ?", bucket, key)
		return err
	})
}

func (m *MetadataStore) ListObjects(bucket, prefix, marker, delimiter string, maxKeys int) (*ListObjectsResult, error) {
	result := &ListObjectsResult{
		Name:      bucket,
		Prefix:    prefix,
		Delimiter: delimiter,
		MaxKeys:   maxKeys,
	}

	query := "SELECT bucket, key, size, etag, content_type, last_modified, storage_path FROM objects WHERE bucket = ?"
	args := []interface{}{bucket}

	if prefix != "" {
		query += " AND key LIKE ?"
		args = append(args, prefix+"%")
	}
	if marker != "" {
		query += " AND key > ?"
		args = append(args, marker)
	}

	query += " ORDER BY key LIMIT ?"
	args = append(args, maxKeys+1)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefixSet := make(map[string]bool)
	for rows.Next() {
		var obj Object
		if err := rows.Scan(&obj.Bucket, &obj.Key, &obj.Size, &obj.ETag, &obj.ContentType, &obj.LastModified, &obj.StoragePath); err != nil {
			return nil, err
		}

		// 处理分隔符
		if delimiter != "" && prefix != "" {
			rest := strings.TrimPrefix(obj.Key, prefix)
			if idx := strings.Index(rest, delimiter); idx >= 0 {
				commonPrefix := prefix + rest[:idx+1]
				if !prefixSet[commonPrefix] {
					prefixSet[commonPrefix] = true
					result.CommonPrefixes = append(result.CommonPrefixes, commonPrefix)
				}
				continue
			}
		}

		if len(result.Contents) < maxKeys {
			result.Contents = append(result.Contents, obj)
		} else {
			result.IsTruncated = true
			break
		}
	}

	if len(result.Contents) > 0 {
		result.NextMarker = result.Contents[len(result.Contents)-1].Key
	}
	result.KeyCount = len(result.Contents)

	return result, nil
}

// === Multipart Upload 操作 ===

func (m *MetadataStore) CreateMultipartUpload(upload *MultipartUpload) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec(`
			INSERT INTO multipart_uploads (upload_id, bucket, key, initiated, content_type)
			VALUES (?, ?, ?, ?, ?)`,
			upload.UploadID, upload.Bucket, upload.Key, upload.Initiated, upload.ContentType,
		)
		return err
	})
}

func (m *MetadataStore) GetMultipartUpload(uploadID string) (*MultipartUpload, error) {
	var upload MultipartUpload
	err := m.db.QueryRow(`
		SELECT upload_id, bucket, key, initiated, content_type
		FROM multipart_uploads WHERE upload_id = ?`, uploadID,
	).Scan(&upload.UploadID, &upload.Bucket, &upload.Key, &upload.Initiated, &upload.ContentType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &upload, err
}

func (m *MetadataStore) DeleteMultipartUpload(uploadID string) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec("DELETE FROM multipart_uploads WHERE upload_id = ?", uploadID)
		return err
	})
}

func (m *MetadataStore) PutPart(part *Part) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec(`
			INSERT OR REPLACE INTO parts (upload_id, part_number, size, etag, modified_at)
			VALUES (?, ?, ?, ?, ?)`,
			part.UploadID, part.PartNumber, part.Size, part.ETag, part.ModifiedAt,
		)
		return err
	})
}

func (m *MetadataStore) ListParts(uploadID string) ([]Part, error) {
	rows, err := m.db.Query(`
		SELECT upload_id, part_number, size, etag, modified_at
		FROM parts WHERE upload_id = ? ORDER BY part_number`, uploadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parts []Part
	for rows.Next() {
		var p Part
		if err := rows.Scan(&p.UploadID, &p.PartNumber, &p.Size, &p.ETag, &p.ModifiedAt); err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return parts, nil
}

func (m *MetadataStore) DeleteParts(uploadID string) error {
	return m.withWriteLock(func() error {
		_, err := m.db.Exec("DELETE FROM parts WHERE upload_id = ?", uploadID)
		return err
	})
}

// escapeLikePattern 转义LIKE模式中的特殊字符
func escapeLikePattern(pattern string) string {
	// 转义 %、_ 和 \ 这些LIKE中的特殊字符
	result := strings.ReplaceAll(pattern, "\\", "\\\\")
	result = strings.ReplaceAll(result, "%", "\\%")
	result = strings.ReplaceAll(result, "_", "\\_")
	return result
}

// SearchObjects 模糊搜索对象（按文件名关键字）
func (m *MetadataStore) SearchObjects(bucket, keyword string, maxResults int) ([]Object, error) {
	if maxResults <= 0 {
		maxResults = 100
	}
	if maxResults > 1000 {
		maxResults = 1000 // 限制最大结果数
	}

	// 转义关键字中的特殊字符，防止SQL注入
	escapedKeyword := escapeLikePattern(keyword)

	query := "SELECT bucket, key, size, etag, content_type, last_modified, storage_path FROM objects WHERE bucket = ? AND key LIKE ? ESCAPE '\\' ORDER BY key LIMIT ?"
	// 使用 %keyword% 实现模糊匹配
	args := []interface{}{bucket, "%" + escapedKeyword + "%", maxResults}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objects []Object
	for rows.Next() {
		var obj Object
		if err := rows.Scan(&obj.Bucket, &obj.Key, &obj.Size, &obj.ETag, &obj.ContentType, &obj.LastModified, &obj.StoragePath); err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}
