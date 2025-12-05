package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEscapeLikePattern 测试LIKE模式转义函数
func TestEscapeLikePattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"普通字符串", "hello", "hello"},
		{"包含百分号", "100%", "100\\%"},
		{"包含下划线", "hello_world", "hello\\_world"},
		{"包含反斜杠", "path\\to\\file", "path\\\\to\\\\file"},
		{"混合特殊字符", "100%_test\\data", "100\\%\\_test\\\\data"},
		{"空字符串", "", ""},
		{"只有特殊字符", "%_\\", "\\%\\_\\\\"},
		{"SQL注入尝试", "'; DROP TABLE--", "'; DROP TABLE--"},
		{"中文字符", "测试文件", "测试文件"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeLikePattern(tt.input)
			if result != tt.expected {
				t.Errorf("escapeLikePattern(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestMetadataStoreIntegration 元数据存储集成测试
func TestMetadataStoreIntegration(t *testing.T) {
	// 创建临时数据库
	tempDir, err := os.MkdirTemp("", "metadata-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewMetadataStore(dbPath)
	if err != nil {
		t.Fatalf("创建MetadataStore失败: %v", err)
	}
	defer store.Close()

	// 测试创建桶
	err = store.CreateBucket("test-bucket")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 测试获取桶
	bucket, err := store.GetBucket("test-bucket")
	if err != nil {
		t.Fatalf("获取桶失败: %v", err)
	}
	if bucket == nil {
		t.Fatal("桶应该存在")
	}
	if bucket.Name != "test-bucket" {
		t.Errorf("桶名不匹配: got %q, want %q", bucket.Name, "test-bucket")
	}

	// 测试列出桶
	buckets, err := store.ListBuckets()
	if err != nil {
		t.Fatalf("列出桶失败: %v", err)
	}
	if len(buckets) != 1 {
		t.Errorf("桶数量不对: got %d, want 1", len(buckets))
	}

	// 测试不存在的桶
	nonExistent, err := store.GetBucket("non-existent")
	if err != nil {
		t.Fatalf("查询不存在的桶应该不报错: %v", err)
	}
	if nonExistent != nil {
		t.Error("不存在的桶应该返回nil")
	}
}

// TestObjectOperations 对象操作测试
func TestObjectOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metadata-objects")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewMetadataStore(dbPath)
	if err != nil {
		t.Fatalf("创建MetadataStore失败: %v", err)
	}
	defer store.Close()

	// 先创建桶
	store.CreateBucket("test-bucket")

	// 测试创建对象
	obj := &Object{
		Bucket:      "test-bucket",
		Key:         "test/file.txt",
		Size:        1024,
		ETag:        "abc123",
		ContentType: "text/plain",
		StoragePath: "/path/to/file",
	}
	err = store.PutObject(obj)
	if err != nil {
		t.Fatalf("创建对象失败: %v", err)
	}

	// 测试获取对象
	retrieved, err := store.GetObject("test-bucket", "test/file.txt")
	if err != nil {
		t.Fatalf("获取对象失败: %v", err)
	}
	if retrieved == nil {
		t.Fatal("对象应该存在")
	}
	if retrieved.Size != 1024 {
		t.Errorf("对象大小不匹配: got %d, want 1024", retrieved.Size)
	}

	// 测试删除对象
	err = store.DeleteObject("test-bucket", "test/file.txt")
	if err != nil {
		t.Fatalf("删除对象失败: %v", err)
	}

	// 验证已删除
	deleted, _ := store.GetObject("test-bucket", "test/file.txt")
	if deleted != nil {
		t.Error("对象应该已被删除")
	}
}

// TestSearchObjects 搜索功能测试（验证SQL注入防护）
func TestSearchObjects(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metadata-search")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewMetadataStore(dbPath)
	if err != nil {
		t.Fatalf("创建MetadataStore失败: %v", err)
	}
	defer store.Close()

	store.CreateBucket("test-bucket")

	// 创建测试对象
	testObjects := []struct {
		key string
	}{
		{"file1.txt"},
		{"file2.txt"},
		{"test_file.txt"},
		{"100%complete.txt"},
		{"document.pdf"},
	}

	for _, obj := range testObjects {
		store.PutObject(&Object{
			Bucket:      "test-bucket",
			Key:         obj.key,
			Size:        100,
			ETag:        "test",
			ContentType: "text/plain",
			StoragePath: "/path/" + obj.key,
		})
	}

	// 测试正常搜索
	results, err := store.SearchObjects("test-bucket", "file", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("搜索结果数量不对: got %d, want 3", len(results))
	}

	// 测试特殊字符搜索（应该被正确转义）
	results, err = store.SearchObjects("test-bucket", "100%", 10)
	if err != nil {
		t.Fatalf("搜索特殊字符失败: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("搜索100%%结果数量不对: got %d, want 1", len(results))
	}

	// 测试下划线搜索
	results, err = store.SearchObjects("test-bucket", "_file", 10)
	if err != nil {
		t.Fatalf("搜索下划线失败: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("搜索_file结果数量不对: got %d, want 1", len(results))
	}

	// 测试SQL注入尝试（应该安全处理）
	results, err = store.SearchObjects("test-bucket", "'; DROP TABLE objects; --", 10)
	if err != nil {
		t.Fatalf("SQL注入尝试应该被安全处理: %v", err)
	}
	// 结果应该为空，但不应该有错误
	if len(results) != 0 {
		t.Errorf("SQL注入搜索应该返回空结果")
	}
}
