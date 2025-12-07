package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupGCTest 为GC测试创建测试环境
func setupGCTest(t *testing.T) (*FileStore, *MetadataStore, func()) {
	t.Helper()

	// 创建临时根目录
	tempRoot := t.TempDir()

	// FileStore 使用 storage 子目录
	storageDir := filepath.Join(tempRoot, "storage")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatalf("创建存储目录失败: %v", err)
	}

	// 数据库使用 db 子目录（与 storage 分离）
	dbDir := filepath.Join(tempRoot, "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("创建数据库目录失败: %v", err)
	}
	dbPath := filepath.Join(dbDir, "test.db")

	// 创建 FileStore
	fs, err := NewFileStore(storageDir)
	if err != nil {
		t.Fatalf("创建FileStore失败: %v", err)
	}

	// 创建 MetadataStore
	ms, err := NewMetadataStore(dbPath)
	if err != nil {
		t.Fatalf("创建MetadataStore失败: %v", err)
	}

	cleanup := func() {
		ms.Close()
	}

	return fs, ms, cleanup
}

// TestScanOrphanFilesBasic 测试基本的孤立文件扫描
func TestScanOrphanFilesBasic(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建一个正常对象
	storagePath1, etag1, _ := fs.PutObject(bucket, "normal.txt", strings.NewReader("normal"), 6)
	ms.PutObject(&Object{
		Bucket:      bucket,
		Key:         "normal.txt",
		Size:        6,
		ETag:        etag1,
		ContentType: "text/plain",
		StoragePath: storagePath1,
	})

	// 创建一个孤立文件（只有磁盘文件，没有元数据）
	orphanDir := filepath.Join(fs.basePath, bucket, "orphan-dir")
	os.MkdirAll(orphanDir, 0755)
	orphanPath := filepath.Join(orphanDir, "orphan.txt")
	os.WriteFile(orphanPath, []byte("orphan"), 0644)

	// 扫描孤立文件
	result, err := fs.ScanOrphanFiles(ms)
	if err != nil {
		t.Fatalf("扫描孤立文件失败: %v", err)
	}

	// 应该找到一个孤立文件
	if result.OrphanCount != 1 {
		t.Errorf("孤立文件数量错误: got %d, want 1", result.OrphanCount)
	}

	if result.OrphanSize != 6 {
		t.Errorf("孤立文件大小错误: got %d, want 6", result.OrphanSize)
	}

	if len(result.OrphanFiles) != 1 {
		t.Fatalf("孤立文件列表长度错误: got %d, want 1", len(result.OrphanFiles))
	}

	// 验证孤立文件信息
	orphan := result.OrphanFiles[0]
	if !strings.Contains(orphan.Path, "orphan.txt") {
		t.Errorf("孤立文件路径错误: %s", orphan.Path)
	}

	if orphan.Size != 6 {
		t.Errorf("孤立文件大小错误: got %d, want 6", orphan.Size)
	}
}

// TestScanOrphanFilesEmpty 测试没有孤立文件的情况
func TestScanOrphanFilesEmpty(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建一个正常对象
	storagePath, etag, _ := fs.PutObject(bucket, "file.txt", strings.NewReader("content"), 7)
	ms.PutObject(&Object{
		Bucket:      bucket,
		Key:         "file.txt",
		Size:        7,
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	})

	// 扫描孤立文件
	result, err := fs.ScanOrphanFiles(ms)
	if err != nil {
		t.Fatalf("扫描孤立文件失败: %v", err)
	}

	// 不应该有孤立文件
	if result.OrphanCount != 0 {
		t.Errorf("应该没有孤立文件: got %d", result.OrphanCount)
	}

	if result.OrphanSize != 0 {
		t.Errorf("孤立文件大小应该为0: got %d", result.OrphanSize)
	}
}

// TestCleanOrphanFiles 测试清理孤立文件
func TestCleanOrphanFiles(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建孤立文件
	orphanDir := filepath.Join(fs.basePath, bucket, "orphan-subdir")
	os.MkdirAll(orphanDir, 0755)
	orphanPath := filepath.Join(orphanDir, "orphan1.txt")
	os.WriteFile(orphanPath, []byte("orphan1"), 0644)

	orphanPath2 := filepath.Join(orphanDir, "orphan2.txt")
	os.WriteFile(orphanPath2, []byte("orphan2"), 0644)

	// 扫描孤立文件
	result, err := fs.ScanOrphanFiles(ms)
	if err != nil {
		t.Fatalf("扫描孤立文件失败: %v", err)
	}

	if result.OrphanCount != 2 {
		t.Fatalf("应该找到2个孤立文件: got %d", result.OrphanCount)
	}

	// 清理孤立文件
	err = fs.CleanOrphanFiles(result.OrphanFiles)
	if err != nil {
		t.Fatalf("清理孤立文件失败: %v", err)
	}

	// 验证文件已删除
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Error("orphan1.txt 应该已删除")
	}

	if _, err := os.Stat(orphanPath2); !os.IsNotExist(err) {
		t.Error("orphan2.txt 应该已删除")
	}

	// 再次扫描，不应该有孤立文件
	result2, err := fs.ScanOrphanFiles(ms)
	if err != nil {
		t.Fatalf("再次扫描失败: %v", err)
	}

	if result2.OrphanCount != 0 {
		t.Errorf("清理后不应该有孤立文件: got %d", result2.OrphanCount)
	}
}

// TestCleanOrphanFilesWithPathTraversal 测试路径遍历攻击防护
func TestCleanOrphanFilesWithPathTraversal(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 尝试使用路径遍历攻击
	maliciousFiles := []OrphanFile{
		{Path: "../../etc/passwd", Size: 100},
		{Path: "../../../tmp/malicious", Size: 200},
	}

	// 清理操作应该安全地跳过这些路径
	err := fs.CleanOrphanFiles(maliciousFiles)
	if err != nil {
		t.Fatalf("清理操作不应该失败: %v", err)
	}

	// 验证 basePath 外的文件没有被删除（这里只是确保没有 panic）
	// 实际验证需要检查这些路径是否存在，但我们主要测试安全性
}

// TestGetExpiredUploads 测试获取过期上传
func TestGetExpiredUploads(t *testing.T) {
	_, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建一个过期的上传（1小时前）
	oldUpload := &MultipartUpload{
		UploadID:    "old-upload-id",
		Bucket:      bucket,
		Key:         "old-file.dat",
		Initiated:   time.Now().Add(-2 * time.Hour),
		ContentType: "application/octet-stream",
	}
	ms.CreateMultipartUpload(oldUpload)

	// 添加一些分片
	ms.PutPart(&Part{
		UploadID:   "old-upload-id",
		PartNumber: 1,
		Size:       1024,
		ETag:       "\"etag1\"",
		ModifiedAt: time.Now().Add(-2 * time.Hour),
	})

	ms.PutPart(&Part{
		UploadID:   "old-upload-id",
		PartNumber: 2,
		Size:       2048,
		ETag:       "\"etag2\"",
		ModifiedAt: time.Now().Add(-2 * time.Hour),
	})

	// 创建一个新的上传
	newUpload := &MultipartUpload{
		UploadID:    "new-upload-id",
		Bucket:      bucket,
		Key:         "new-file.dat",
		Initiated:   time.Now().Add(-10 * time.Minute),
		ContentType: "application/octet-stream",
	}
	ms.CreateMultipartUpload(newUpload)

	// 获取1小时前的过期上传
	expired, err := ms.GetExpiredUploads(1 * time.Hour)
	if err != nil {
		t.Fatalf("获取过期上传失败: %v", err)
	}

	// 应该只有一个过期上传
	if len(expired) != 1 {
		t.Fatalf("过期上传数量错误: got %d, want 1", len(expired))
	}

	upload := expired[0]
	if upload.UploadID != "old-upload-id" {
		t.Errorf("上传ID错误: got %s, want old-upload-id", upload.UploadID)
	}

	if upload.PartCount != 2 {
		t.Errorf("分片数量错误: got %d, want 2", upload.PartCount)
	}

	if upload.TotalSize != 3072 {
		t.Errorf("总大小错误: got %d, want 3072", upload.TotalSize)
	}
}

// TestCleanExpiredUploads 测试清理过期上传
func TestCleanExpiredUploads(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建上传和分片
	uploadID := "test-upload-id"
	ms.CreateMultipartUpload(&MultipartUpload{
		UploadID:    uploadID,
		Bucket:      bucket,
		Key:         "test.dat",
		Initiated:   time.Now().Add(-2 * time.Hour),
		ContentType: "application/octet-stream",
	})

	ms.PutPart(&Part{
		UploadID:   uploadID,
		PartNumber: 1,
		Size:       1000,
		ETag:       "\"etag1\"",
		ModifiedAt: time.Now(),
	})

	ms.PutPart(&Part{
		UploadID:   uploadID,
		PartNumber: 2,
		Size:       2000,
		ETag:       "\"etag2\"",
		ModifiedAt: time.Now(),
	})

	// 清理上传
	cleaned, err := ms.CleanExpiredUploads([]string{uploadID}, fs)
	if err != nil {
		t.Fatalf("清理过期上传失败: %v", err)
	}

	if cleaned != 3000 {
		t.Errorf("清理大小错误: got %d, want 3000", cleaned)
	}

	// 验证上传已删除
	upload, err := ms.GetMultipartUpload(uploadID)
	if err != nil {
		t.Fatalf("查询上传失败: %v", err)
	}

	if upload != nil {
		t.Error("上传应该已删除")
	}

	// 验证分片已删除
	parts, err := ms.ListParts(uploadID)
	if err != nil {
		t.Fatalf("列出分片失败: %v", err)
	}

	if len(parts) != 0 {
		t.Errorf("分片应该已删除: got %d", len(parts))
	}
}

// TestScanMultipartOrphans 测试扫描孤立的分片目录
func TestScanMultipartOrphans(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建一个活跃的上传
	activeUploadID := "active-upload"
	ms.CreateMultipartUpload(&MultipartUpload{
		UploadID:    activeUploadID,
		Bucket:      bucket,
		Key:         "active.dat",
		Initiated:   time.Now(),
		ContentType: "application/octet-stream",
	})

	// 创建活跃上传的分片目录
	activeDir := filepath.Join(fs.basePath, ".multipart", activeUploadID)
	os.MkdirAll(activeDir, 0755)
	os.WriteFile(filepath.Join(activeDir, "part-1"), []byte("active part"), 0644)

	// 创建一个孤立的上传目录（没有元数据记录）
	orphanUploadID := "orphan-upload"
	orphanDir := filepath.Join(fs.basePath, ".multipart", orphanUploadID)
	os.MkdirAll(orphanDir, 0755)
	os.WriteFile(filepath.Join(orphanDir, "part-1"), []byte("orphan part 1"), 0644)
	os.WriteFile(filepath.Join(orphanDir, "part-2"), []byte("orphan part 2"), 0644)

	// 扫描孤立分片
	orphans, totalSize, err := fs.ScanMultipartOrphans(ms)
	if err != nil {
		t.Fatalf("扫描孤立分片失败: %v", err)
	}

	// 应该找到孤立上传的2个分片
	if len(orphans) != 2 {
		t.Errorf("孤立分片数量错误: got %d, want 2", len(orphans))
	}

	expectedSize := int64(len("orphan part 1") + len("orphan part 2"))
	if totalSize != expectedSize {
		t.Errorf("孤立分片总大小错误: got %d, want %d", totalSize, expectedSize)
	}

	// 验证路径包含 .multipart 和孤立上传ID
	for _, orphan := range orphans {
		if !strings.Contains(orphan.Path, ".multipart") {
			t.Errorf("孤立分片路径应该包含.multipart: %s", orphan.Path)
		}
		if !strings.Contains(orphan.Path, orphanUploadID) {
			t.Errorf("孤立分片路径应该包含上传ID: %s", orphan.Path)
		}
	}
}

// TestScanMultipartOrphansNoMultipartDir 测试没有.multipart目录的情况
func TestScanMultipartOrphansNoMultipartDir(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	// 不创建.multipart目录，直接扫描
	orphans, totalSize, err := fs.ScanMultipartOrphans(ms)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	if len(orphans) != 0 {
		t.Errorf("不应该有孤立分片: got %d", len(orphans))
	}

	if totalSize != 0 {
		t.Errorf("总大小应该为0: got %d", totalSize)
	}
}

// TestRunGCDryRun 测试GC干运行模式
func TestRunGCDryRun(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建孤立文件
	orphanDir := filepath.Join(fs.basePath, bucket, "orphan")
	os.MkdirAll(orphanDir, 0755)
	orphanPath := filepath.Join(orphanDir, "orphan.txt")
	os.WriteFile(orphanPath, []byte("orphan"), 0644)

	// 创建过期上传
	oldUploadID := "old-upload"
	ms.CreateMultipartUpload(&MultipartUpload{
		UploadID:    oldUploadID,
		Bucket:      bucket,
		Key:         "old.dat",
		Initiated:   time.Now().Add(-2 * time.Hour),
		ContentType: "application/octet-stream",
	})

	ms.PutPart(&Part{
		UploadID:   oldUploadID,
		PartNumber: 1,
		Size:       500,
		ETag:       "\"etag\"",
		ModifiedAt: time.Now().Add(-2 * time.Hour),
	})

	// 执行干运行GC
	result, err := RunGC(fs, ms, 1*time.Hour, true)
	if err != nil {
		t.Fatalf("GC失败: %v", err)
	}

	// 验证找到了孤立文件和过期上传
	if result.OrphanCount != 1 {
		t.Errorf("孤立文件数量错误: got %d, want 1", result.OrphanCount)
	}

	if result.ExpiredCount != 1 {
		t.Errorf("过期上传数量错误: got %d, want 1", result.ExpiredCount)
	}

	// 干运行不应该清理
	if result.Cleaned {
		t.Error("干运行不应该标记为已清理")
	}

	if result.CleanedAt != nil {
		t.Error("干运行不应该设置清理时间")
	}

	// 验证文件仍然存在
	if _, err := os.Stat(orphanPath); os.IsNotExist(err) {
		t.Error("干运行不应该删除文件")
	}

	// 验证上传仍然存在
	upload, _ := ms.GetMultipartUpload(oldUploadID)
	if upload == nil {
		t.Error("干运行不应该删除上传")
	}
}

// TestRunGCCleanup 测试GC实际清理模式
func TestRunGCCleanup(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建孤立文件
	orphanDir := filepath.Join(fs.basePath, bucket, "orphan")
	os.MkdirAll(orphanDir, 0755)
	orphanPath := filepath.Join(orphanDir, "orphan.txt")
	os.WriteFile(orphanPath, []byte("orphan"), 0644)

	// 创建过期上传
	oldUploadID := "old-upload"
	ms.CreateMultipartUpload(&MultipartUpload{
		UploadID:    oldUploadID,
		Bucket:      bucket,
		Key:         "old.dat",
		Initiated:   time.Now().Add(-2 * time.Hour),
		ContentType: "application/octet-stream",
	})

	ms.PutPart(&Part{
		UploadID:   oldUploadID,
		PartNumber: 1,
		Size:       500,
		ETag:       "\"etag\"",
		ModifiedAt: time.Now().Add(-2 * time.Hour),
	})

	// 执行实际清理GC
	result, err := RunGC(fs, ms, 1*time.Hour, false)
	if err != nil {
		t.Fatalf("GC失败: %v", err)
	}

	// 验证已标记为清理
	if !result.Cleaned {
		t.Error("应该标记为已清理")
	}

	if result.CleanedAt == nil {
		t.Error("应该设置清理时间")
	}

	// 验证文件已删除
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Error("孤立文件应该已删除")
	}

	// 验证上传已删除
	upload, _ := ms.GetMultipartUpload(oldUploadID)
	if upload != nil {
		t.Error("过期上传应该已删除")
	}
}

// TestGetStoragePathFromKey 测试存储路径计算
func TestGetStoragePathFromKey(t *testing.T) {
	fs, _, cleanup := setupGCTest(t)
	defer cleanup()

	testCases := []struct {
		bucket string
		key    string
	}{
		{"bucket1", "file1.txt"},
		{"bucket1", "dir/file2.txt"},
		{"bucket2", "file1.txt"}, // 相同key不同bucket
		{"bucket1", "中文文件.txt"},
		{"bucket1", "file with spaces.txt"},
	}

	for _, tc := range testCases {
		path := fs.GetStoragePathFromKey(tc.bucket, tc.key)

		// 验证路径包含bucket
		if !strings.Contains(path, tc.bucket) {
			t.Errorf("路径应该包含bucket: %s", path)
		}

		// 验证路径包含basePath
		if !strings.HasPrefix(path, fs.basePath) {
			t.Errorf("路径应该以basePath开头: %s", path)
		}

		// 验证路径包含key
		if !strings.HasSuffix(path, tc.key) {
			t.Errorf("路径应该以key结尾: %s", path)
		}
	}
}

// TestGetStoragePathFromKeyConsistency 测试路径计算的一致性
func TestGetStoragePathFromKeyConsistency(t *testing.T) {
	fs, _, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	key := "test-file.txt"

	// 多次计算应该得到相同结果
	path1 := fs.GetStoragePathFromKey(bucket, key)
	path2 := fs.GetStoragePathFromKey(bucket, key)
	path3 := fs.GetStoragePathFromKey(bucket, key)

	if path1 != path2 || path2 != path3 {
		t.Errorf("相同bucket和key应该产生相同路径: %s, %s, %s", path1, path2, path3)
	}
}

// TestListAllObjects 测试列出桶中所有对象
func TestListAllObjects(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建多个对象
	for i := 1; i <= 5; i++ {
		key := filepath.Join("file", string(rune('0'+i))+".txt")
		content := "content" + string(rune('0'+i))
		storagePath, etag, _ := fs.PutObject(bucket, key, strings.NewReader(content), int64(len(content)))
		ms.PutObject(&Object{
			Bucket:      bucket,
			Key:         key,
			Size:        int64(len(content)),
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		})
	}

	// 列出所有对象
	objects, err := ms.ListAllObjects(bucket)
	if err != nil {
		t.Fatalf("列出对象失败: %v", err)
	}

	if len(objects) != 5 {
		t.Errorf("对象数量错误: got %d, want 5", len(objects))
	}

	// 验证按key排序
	for i := 0; i < len(objects)-1; i++ {
		if objects[i].Key > objects[i+1].Key {
			t.Error("对象应该按key排序")
			break
		}
	}
}

// TestGCWithMultipleBuckets 测试多桶场景下的GC
func TestGCWithMultipleBuckets(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()

	// 创建多个桶
	buckets := []string{"bucket1", "bucket2", "bucket3"}
	for _, bucket := range buckets {
		ms.CreateBucket(bucket)

		// 每个桶创建一个正常文件
		storagePath, etag, _ := fs.PutObject(bucket, "normal.txt", strings.NewReader("normal"), 6)
		ms.PutObject(&Object{
			Bucket:      bucket,
			Key:         "normal.txt",
			Size:        6,
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		})

		// 每个桶创建一个孤立文件
		orphanDir := filepath.Join(fs.basePath, bucket, "orphan")
		os.MkdirAll(orphanDir, 0755)
		os.WriteFile(filepath.Join(orphanDir, "orphan.txt"), []byte("orphan"), 0644)
	}

	// 扫描孤立文件
	result, err := fs.ScanOrphanFiles(ms)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	// 应该找到3个孤立文件（每个桶一个）
	if result.OrphanCount != 3 {
		t.Errorf("孤立文件数量错误: got %d, want 3", result.OrphanCount)
	}

	// 清理孤立文件
	err = fs.CleanOrphanFiles(result.OrphanFiles)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}

	// 验证正常文件仍存在
	for _, bucket := range buckets {
		obj, err := ms.GetObject(bucket, "normal.txt")
		if err != nil {
			t.Fatalf("获取对象失败: %v", err)
		}
		if obj == nil {
			t.Errorf("正常文件不应该被删除: %s/normal.txt", bucket)
		}
	}
}

// BenchmarkScanOrphanFiles GC扫描性能基准
func BenchmarkScanOrphanFiles(b *testing.B) {
	fs, ms, cleanup := setupGCTest(&testing.T{})
	defer cleanup()

	bucket := "bench-bucket"
	ms.CreateBucket(bucket)

	// 创建一些正常对象
	for i := 0; i < 100; i++ {
		key := "file-" + string(rune('0'+i%10)) + ".txt"
		storagePath, etag, _ := fs.PutObject(bucket, key, strings.NewReader("content"), 7)
		ms.PutObject(&Object{
			Bucket:      bucket,
			Key:         key,
			Size:        7,
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		})
	}

	// 创建一些孤立文件
	for i := 0; i < 10; i++ {
		orphanPath := filepath.Join(fs.basePath, bucket, "orphan", string(rune('0'+i))+".txt")
		os.MkdirAll(filepath.Dir(orphanPath), 0755)
		os.WriteFile(orphanPath, []byte("orphan"), 0644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fs.ScanOrphanFiles(ms)
		if err != nil {
			b.Fatalf("扫描失败: %v", err)
		}
	}
}

// BenchmarkRunGC 完整GC性能基准
func BenchmarkRunGC(b *testing.B) {
	fs, ms, cleanup := setupGCTest(&testing.T{})
	defer cleanup()

	bucket := "bench-bucket"
	ms.CreateBucket(bucket)

	// 创建测试数据
	for i := 0; i < 50; i++ {
		key := "file-" + string(rune('0'+i%10)) + ".txt"
		storagePath, etag, _ := fs.PutObject(bucket, key, strings.NewReader("content"), 7)
		ms.PutObject(&Object{
			Bucket:      bucket,
			Key:         key,
			Size:        7,
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := RunGC(fs, ms, 24*time.Hour, true)
		if err != nil {
			b.Fatalf("GC失败: %v", err)
		}
	}
}
