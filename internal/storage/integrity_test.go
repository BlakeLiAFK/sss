package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupIntegrityTest 为完整性测试创建测试环境
func setupIntegrityTest(t *testing.T) (*FileStore, *MetadataStore, func()) {
	t.Helper()

	// 创建FileStore
	fsCleanup := func() {}
	tempDir := t.TempDir()
	fs, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("创建FileStore失败: %v", err)
	}

	// 创建MetadataStore
	dbPath := filepath.Join(tempDir, "test.db")
	ms, err := NewMetadataStore(dbPath)
	if err != nil {
		fsCleanup()
		t.Fatalf("创建MetadataStore失败: %v", err)
	}

	cleanup := func() {
		ms.Close()
		fsCleanup()
	}

	return fs, ms, cleanup
}

// TestCheckIntegrityBasic 测试基本的完整性检查
func TestCheckIntegrityBasic(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建一个正常的对象
	data := []byte("test data")
	storagePath, etag, _ := fs.PutObject(bucket, "file1.txt", strings.NewReader(string(data)), int64(len(data)))

	obj := &Object{
		Bucket:      bucket,
		Key:         "file1.txt",
		Size:        int64(len(data)),
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	ms.PutObject(obj)

	// 执行完整性检查
	result, err := CheckIntegrity(fs, ms, true, 0)
	if err != nil {
		t.Fatalf("完整性检查失败: %v", err)
	}

	// 应该没有问题
	if result.IssuesFound != 0 {
		t.Errorf("不应该发现问题: found %d issues", result.IssuesFound)
	}

	if result.TotalChecked != 1 {
		t.Errorf("应该检查1个对象: checked %d", result.TotalChecked)
	}
}

// TestCheckIntegrityMissingFile 测试检测缺失文件
func TestCheckIntegrityMissingFile(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建元数据但不创建实际文件
	obj := &Object{
		Bucket:      bucket,
		Key:         "missing.txt",
		Size:        100,
		ETag:        "fake-etag",
		ContentType: "text/plain",
		StoragePath: filepath.Join(fs.basePath, bucket, "missing.txt"),
	}
	ms.PutObject(obj)

	// 执行完整性检查
	result, err := CheckIntegrity(fs, ms, false, 0)
	if err != nil {
		t.Fatalf("完整性检查失败: %v", err)
	}

	// 应该发现1个缺失文件问题
	if result.IssuesFound != 1 {
		t.Errorf("应该发现1个问题: found %d", result.IssuesFound)
	}

	if result.MissingFiles != 1 {
		t.Errorf("应该发现1个缺失文件: found %d", result.MissingFiles)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("应该有1个问题记录: got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.IssueType != "missing_file" {
		t.Errorf("问题类型应该是missing_file: got %s", issue.IssueType)
	}

	if !issue.Repairable {
		t.Error("缺失文件问题应该是可修复的")
	}
}

// TestCheckIntegrityEtagMismatch 测试检测ETag不匹配
func TestCheckIntegrityEtagMismatch(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建对象
	data := []byte("test data")
	storagePath, _, _ := fs.PutObject(bucket, "file1.txt", strings.NewReader(string(data)), int64(len(data)))

	// 存储错误的ETag
	obj := &Object{
		Bucket:      bucket,
		Key:         "file1.txt",
		Size:        int64(len(data)),
		ETag:        "\"wrong-etag\"",
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	ms.PutObject(obj)

	// 执行完整性检查（启用ETag验证）
	result, err := CheckIntegrity(fs, ms, true, 0)
	if err != nil {
		t.Fatalf("完整性检查失败: %v", err)
	}

	// 应该发现ETag不匹配问题
	if result.EtagMismatches != 1 {
		t.Errorf("应该发现1个ETag不匹配: found %d", result.EtagMismatches)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("应该有1个问题记录: got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.IssueType != "etag_mismatch" {
		t.Errorf("问题类型应该是etag_mismatch: got %s", issue.IssueType)
	}

	if !issue.Repairable {
		t.Error("ETag不匹配问题应该是可修复的")
	}
}

// TestCheckIntegrityWithLimit 测试限制检查数量
func TestCheckIntegrityWithLimit(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建多个对象
	for i := 1; i <= 10; i++ {
		key := "missing.txt"
		if i > 1 {
			key = "missing.txt" + string(rune('0'+i))
		}
		obj := &Object{
			Bucket:      bucket,
			Key:         key,
			Size:        100,
			ETag:        "fake-etag",
			ContentType: "text/plain",
			StoragePath: filepath.Join(fs.basePath, bucket, key),
		}
		ms.PutObject(obj)
	}

	// 限制只检查前5个
	result, err := CheckIntegrity(fs, ms, false, 5)
	if err != nil {
		t.Fatalf("完整性检查失败: %v", err)
	}

	if result.TotalChecked != 5 {
		t.Errorf("应该检查5个对象: checked %d", result.TotalChecked)
	}

	if result.IssuesFound != 5 {
		t.Errorf("应该发现5个问题: found %d", result.IssuesFound)
	}
}

// TestCheckIntegrityMultipleBuckets 测试多桶检查
func TestCheckIntegrityMultipleBuckets(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	// 创建多个桶
	buckets := []string{"bucket1", "bucket2", "bucket3"}
	for _, bucket := range buckets {
		ms.CreateBucket(bucket)

		// 每个桶创建一个对象
		data := []byte("test data")
		storagePath, etag, _ := fs.PutObject(bucket, "file.txt", strings.NewReader(string(data)), int64(len(data)))

		obj := &Object{
			Bucket:      bucket,
			Key:         "file.txt",
			Size:        int64(len(data)),
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		}
		ms.PutObject(obj)
	}

	// 执行完整性检查
	result, err := CheckIntegrity(fs, ms, true, 0)
	if err != nil {
		t.Fatalf("完整性检查失败: %v", err)
	}

	if result.TotalChecked != 3 {
		t.Errorf("应该检查3个对象: checked %d", result.TotalChecked)
	}

	if result.IssuesFound != 0 {
		t.Errorf("不应该发现问题: found %d", result.IssuesFound)
	}
}

// TestRepairIntegrityMissingFile 测试修复缺失文件问题
func TestRepairIntegrityMissingFile(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建元数据但不创建实际文件
	obj := &Object{
		Bucket:      bucket,
		Key:         "missing.txt",
		Size:        100,
		ETag:        "fake-etag",
		ContentType: "text/plain",
		StoragePath: filepath.Join(fs.basePath, bucket, "missing.txt"),
	}
	ms.PutObject(obj)

	// 检查问题
	checkResult, _ := CheckIntegrity(fs, ms, false, 0)
	if checkResult.IssuesFound == 0 {
		t.Fatal("应该发现问题")
	}

	// 修复问题
	repairResult, err := RepairIntegrity(fs, ms, checkResult.Issues)
	if err != nil {
		t.Fatalf("修复失败: %v", err)
	}

	if repairResult.RepairedCount != 1 {
		t.Errorf("应该修复1个问题: repaired %d", repairResult.RepairedCount)
	}

	// 验证元数据已删除
	retrievedObj, _ := ms.GetObject(bucket, "missing.txt")
	if retrievedObj != nil {
		t.Error("缺失文件的元数据应该已被删除")
	}

	// 再次检查应该没有问题
	checkResult2, _ := CheckIntegrity(fs, ms, false, 0)
	if checkResult2.IssuesFound != 0 {
		t.Errorf("修复后不应该有问题: found %d", checkResult2.IssuesFound)
	}
}

// TestRepairIntegrityEtagMismatch 测试修复ETag不匹配问题
func TestRepairIntegrityEtagMismatch(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建对象
	data := []byte("test data for etag")
	storagePath, _, _ := fs.PutObject(bucket, "file.txt", strings.NewReader(string(data)), int64(len(data)))

	// 存储错误的ETag
	obj := &Object{
		Bucket:      bucket,
		Key:         "file.txt",
		Size:        int64(len(data)),
		ETag:        "\"wrong-etag\"",
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	ms.PutObject(obj)

	// 检查问题
	checkResult, _ := CheckIntegrity(fs, ms, true, 0)
	if checkResult.EtagMismatches == 0 {
		t.Fatal("应该发现ETag不匹配问题")
	}

	// 修复问题
	repairResult, err := RepairIntegrity(fs, ms, checkResult.Issues)
	if err != nil {
		t.Fatalf("修复失败: %v", err)
	}

	if repairResult.RepairedCount != 1 {
		t.Errorf("应该修复1个问题: repaired %d", repairResult.RepairedCount)
	}

	// 再次检查应该没有问题
	checkResult2, _ := CheckIntegrity(fs, ms, true, 0)
	if checkResult2.EtagMismatches != 0 {
		t.Errorf("修复后不应该有ETag不匹配: found %d", checkResult2.EtagMismatches)
	}
}

// TestCalculateFileEtag 测试计算文件ETag
func TestCalculateFileEtag(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")

	// 创建测试文件
	data := []byte("test data for etag calculation")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 计算ETag
	etag, err := calculateFileEtag(filePath)
	if err != nil {
		t.Fatalf("计算ETag失败: %v", err)
	}

	// 验证ETag是32字符的十六进制字符串（MD5）
	if len(etag) != 32 {
		t.Errorf("ETag长度应该是32: got %d", len(etag))
	}

	// 验证是十六进制
	for _, c := range etag {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("ETag应该是十六进制: got %s", etag)
			break
		}
	}

	// 同一文件多次计算应该得到相同结果
	etag2, _ := calculateFileEtag(filePath)
	if etag != etag2 {
		t.Errorf("同一文件的ETag应该一致: %s != %s", etag, etag2)
	}

	// 不同内容应该产生不同ETag
	filePath2 := filepath.Join(tempDir, "test2.txt")
	os.WriteFile(filePath2, []byte("different data"), 0644)
	etag3, _ := calculateFileEtag(filePath2)
	if etag == etag3 {
		t.Error("不同内容应该产生不同的ETag")
	}
}

// TestCalculateFileEtagErrors 测试ETag计算的错误处理
func TestCalculateFileEtagErrors(t *testing.T) {
	// 不存在的文件
	_, err := calculateFileEtag("/nonexistent/file.txt")
	if err == nil {
		t.Error("不存在的文件应该返回错误")
	}

	// 目录而非文件
	tempDir := t.TempDir()
	_, err = calculateFileEtag(tempDir)
	if err == nil {
		t.Error("目录应该返回错误")
	}
}

// TestTrimQuotes 测试去除引号功能
func TestTrimQuotes(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{`"abc"`, "abc"},
		{`"hello world"`, "hello world"},
		{`""`, ""},
		{`"`, `"`},
		{`abc`, "abc"},
		{`"abc`, `"abc`},
		{`abc"`, `abc"`},
		{``, ``},
	}

	for _, tc := range testCases {
		result := trimQuotes(tc.input)
		if result != tc.expected {
			t.Errorf("trimQuotes(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// TestIntegrityResultFields 测试IntegrityResult的所有字段
func TestIntegrityResultFields(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建各种问题场景
	// 1. 缺失文件
	ms.PutObject(&Object{
		Bucket:      bucket,
		Key:         "missing.txt",
		Size:        100,
		ETag:        "fake",
		ContentType: "text/plain",
		StoragePath: filepath.Join(fs.basePath, bucket, "missing.txt"),
	})

	// 2. ETag不匹配
	data := []byte("test")
	storagePath, _, _ := fs.PutObject(bucket, "etag-mismatch.txt", strings.NewReader(string(data)), int64(len(data)))
	ms.PutObject(&Object{
		Bucket:      bucket,
		Key:         "etag-mismatch.txt",
		Size:        int64(len(data)),
		ETag:        "\"wrong\"",
		ContentType: "text/plain",
		StoragePath: storagePath,
	})

	// 执行检查
	result, err := CheckIntegrity(fs, ms, true, 0)
	if err != nil {
		t.Fatalf("检查失败: %v", err)
	}

	// 验证所有字段
	if result.TotalChecked != 2 {
		t.Errorf("TotalChecked应该是2: got %d", result.TotalChecked)
	}

	if result.IssuesFound != 2 {
		t.Errorf("IssuesFound应该是2: got %d", result.IssuesFound)
	}

	if result.MissingFiles != 1 {
		t.Errorf("MissingFiles应该是1: got %d", result.MissingFiles)
	}

	if result.EtagMismatches != 1 {
		t.Errorf("EtagMismatches应该是1: got %d", result.EtagMismatches)
	}

	if result.Duration <= 0 {
		t.Error("Duration应该大于0")
	}

	if result.CheckedAt.IsZero() {
		t.Error("CheckedAt不应该为零值")
	}

	if result.Repaired {
		t.Error("Repaired应该为false（未修复）")
	}

	if result.RepairedCount != 0 {
		t.Errorf("RepairedCount应该是0: got %d", result.RepairedCount)
	}
}

// TestCheckIntegrityWithoutEtagVerification 测试不验证ETag的检查
func TestCheckIntegrityWithoutEtagVerification(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()

	bucket := "test-bucket"
	ms.CreateBucket(bucket)

	// 创建ETag错误的对象
	data := []byte("test")
	storagePath, _, _ := fs.PutObject(bucket, "file.txt", strings.NewReader(string(data)), int64(len(data)))
	ms.PutObject(&Object{
		Bucket:      bucket,
		Key:         "file.txt",
		Size:        int64(len(data)),
		ETag:        "\"wrong\"",
		ContentType: "text/plain",
		StoragePath: storagePath,
	})

	// 不验证ETag
	result, err := CheckIntegrity(fs, ms, false, 0)
	if err != nil {
		t.Fatalf("检查失败: %v", err)
	}

	// 不应该发现ETag问题
	if result.EtagMismatches != 0 {
		t.Errorf("不验证ETag时不应该发现ETag问题: found %d", result.EtagMismatches)
	}
}

// BenchmarkCheckIntegrity 完整性检查性能基准
func BenchmarkCheckIntegrity(b *testing.B) {
	fs, ms, cleanup := setupIntegrityTest(&testing.T{})
	defer cleanup()

	bucket := "bench-bucket"
	ms.CreateBucket(bucket)

	// 创建一些测试对象
	for i := 0; i < 100; i++ {
		key := "file-" + string(rune('0'+i%10)) + ".txt"
		data := []byte("test data")
		storagePath, etag, _ := fs.PutObject(bucket, key, strings.NewReader(string(data)), int64(len(data)))
		ms.PutObject(&Object{
			Bucket:      bucket,
			Key:         key,
			Size:        int64(len(data)),
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckIntegrity(fs, ms, false, 0)
	}
}
