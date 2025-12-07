package storage

import (
	"strings"
	"testing"
	"time"
)

// setupStatsTest 为统计测试创建 MetadataStore 和 FileStore
func setupStatsTest(t *testing.T) (*MetadataStore, *FileStore, func()) {
	t.Helper()

	ms, cleanup1 := setupMetadataStore(t)
	fs, cleanup2 := setupFileStore(t)

	cleanup := func() {
		cleanup1()
		cleanup2()
	}

	return ms, fs, cleanup
}

// TestGetStorageStatsEmpty 测试空数据统计
func TestGetStorageStatsEmpty(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	stats, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	if stats.TotalBuckets != 0 {
		t.Errorf("空数据应该有0个桶: got %d", stats.TotalBuckets)
	}

	if stats.TotalObjects != 0 {
		t.Errorf("空数据应该有0个对象: got %d", stats.TotalObjects)
	}

	if stats.TotalSize != 0 {
		t.Errorf("空数据总大小应该是0: got %d", stats.TotalSize)
	}

	if len(stats.BucketStats) != 0 {
		t.Errorf("空数据不应该有桶统计: got %d", len(stats.BucketStats))
	}

	if len(stats.TypeStats) != 0 {
		t.Errorf("空数据不应该有类型统计: got %d", len(stats.TypeStats))
	}
}

// TestGetStorageStatsSingleBucket 测试单桶统计
func TestGetStorageStatsSingleBucket(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	// 创建一个桶
	err := ms.CreateBucket("test-bucket")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 添加一些对象
	objects := []Object{
		{
			Bucket:       "test-bucket",
			Key:          "file1.txt",
			Size:         1024,
			ETag:         "etag1",
			ContentType:  "text/plain",
			StoragePath:  "/path/file1",
			LastModified: time.Now().UTC(),
		},
		{
			Bucket:       "test-bucket",
			Key:          "file2.jpg",
			Size:         2048,
			ETag:         "etag2",
			ContentType:  "image/jpeg",
			StoragePath:  "/path/file2",
			LastModified: time.Now().UTC(),
		},
		{
			Bucket:       "test-bucket",
			Key:          "file3.pdf",
			Size:         4096,
			ETag:         "etag3",
			ContentType:  "application/pdf",
			StoragePath:  "/path/file3",
			LastModified: time.Now().UTC(),
		},
	}

	for _, obj := range objects {
		if err := ms.PutObject(&obj); err != nil {
			t.Fatalf("添加对象失败: %v", err)
		}
	}

	stats, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	// 验证总体统计
	if stats.TotalBuckets != 1 {
		t.Errorf("桶总数错误: got %d, want 1", stats.TotalBuckets)
	}

	if stats.TotalObjects != 3 {
		t.Errorf("对象总数错误: got %d, want 3", stats.TotalObjects)
	}

	expectedSize := int64(1024 + 2048 + 4096)
	if stats.TotalSize != expectedSize {
		t.Errorf("总大小错误: got %d, want %d", stats.TotalSize, expectedSize)
	}

	// 验证桶统计
	if len(stats.BucketStats) != 1 {
		t.Fatalf("桶统计数量错误: got %d, want 1", len(stats.BucketStats))
	}

	bucketStat := stats.BucketStats[0]
	if bucketStat.Name != "test-bucket" {
		t.Errorf("桶名错误: got %s, want test-bucket", bucketStat.Name)
	}

	if bucketStat.ObjectCount != 3 {
		t.Errorf("桶对象数量错误: got %d, want 3", bucketStat.ObjectCount)
	}

	if bucketStat.TotalSize != expectedSize {
		t.Errorf("桶总大小错误: got %d, want %d", bucketStat.TotalSize, expectedSize)
	}

	// 验证文件类型统计
	if len(stats.TypeStats) != 3 {
		t.Fatalf("文件类型统计数量错误: got %d, want 3", len(stats.TypeStats))
	}

	// 验证类型统计按大小降序排列
	for i := 0; i < len(stats.TypeStats)-1; i++ {
		if stats.TypeStats[i].TotalSize < stats.TypeStats[i+1].TotalSize {
			t.Error("类型统计应该按总大小降序排列")
		}
	}
}

// TestGetStorageStatsMultipleBuckets 测试多桶统计
func TestGetStorageStatsMultipleBuckets(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	// 创建多个桶
	buckets := []string{"bucket1", "bucket2", "bucket3"}
	for _, name := range buckets {
		if err := ms.CreateBucket(name); err != nil {
			t.Fatalf("创建桶失败: %v", err)
		}
	}

	// bucket1: 2个对象，共3KB
	ms.PutObject(&Object{
		Bucket: "bucket1", Key: "a.txt", Size: 1024,
		ETag: "e1", ContentType: "text/plain", StoragePath: "/path/a",
		LastModified: time.Now().UTC(),
	})
	ms.PutObject(&Object{
		Bucket: "bucket1", Key: "b.txt", Size: 2048,
		ETag: "e2", ContentType: "text/plain", StoragePath: "/path/b",
		LastModified: time.Now().UTC(),
	})

	// bucket2: 1个对象，5KB
	ms.PutObject(&Object{
		Bucket: "bucket2", Key: "c.jpg", Size: 5120,
		ETag: "e3", ContentType: "image/jpeg", StoragePath: "/path/c",
		LastModified: time.Now().UTC(),
	})

	// bucket3: 空桶
	// (不添加对象)

	stats, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	// 验证总体统计
	if stats.TotalBuckets != 3 {
		t.Errorf("桶总数错误: got %d, want 3", stats.TotalBuckets)
	}

	if stats.TotalObjects != 3 {
		t.Errorf("对象总数错误: got %d, want 3", stats.TotalObjects)
	}

	expectedSize := int64(1024 + 2048 + 5120)
	if stats.TotalSize != expectedSize {
		t.Errorf("总大小错误: got %d, want %d", stats.TotalSize, expectedSize)
	}

	// 验证桶统计包含所有桶（包括空桶）
	if len(stats.BucketStats) != 3 {
		t.Fatalf("桶统计数量错误: got %d, want 3", len(stats.BucketStats))
	}

	// 验证桶统计按总大小降序排列
	if stats.BucketStats[0].Name != "bucket2" {
		t.Errorf("第一个应该是 bucket2 (最大): got %s", stats.BucketStats[0].Name)
	}

	if stats.BucketStats[0].TotalSize != 5120 {
		t.Errorf("bucket2 大小错误: got %d, want 5120", stats.BucketStats[0].TotalSize)
	}

	// 验证空桶
	var emptyBucket *BucketStat
	for _, bs := range stats.BucketStats {
		if bs.Name == "bucket3" {
			emptyBucket = &bs
			break
		}
	}

	if emptyBucket == nil {
		t.Fatal("空桶应该出现在统计中")
	}

	if emptyBucket.ObjectCount != 0 {
		t.Errorf("空桶对象数应该是0: got %d", emptyBucket.ObjectCount)
	}

	if emptyBucket.TotalSize != 0 {
		t.Errorf("空桶大小应该是0: got %d", emptyBucket.TotalSize)
	}
}

// TestGetStorageStatsPublicBucket 测试公开桶标识
func TestGetStorageStatsPublicBucket(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	// 创建公开桶
	err := ms.CreateBucket("public-bucket")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 设置为公开
	err = ms.UpdateBucketPublic("public-bucket", true)
	if err != nil {
		t.Fatalf("设置公开失败: %v", err)
	}

	// 创建私有桶
	err = ms.CreateBucket("private-bucket")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	stats, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	// 验证桶的公开状态
	for _, bs := range stats.BucketStats {
		if bs.Name == "public-bucket" {
			if !bs.IsPublic {
				t.Error("public-bucket 应该是公开的")
			}
		}
		if bs.Name == "private-bucket" {
			if bs.IsPublic {
				t.Error("private-bucket 应该是私有的")
			}
		}
	}
}

// TestGetStorageStatsTypeLimit 测试类型统计限制
func TestGetStorageStatsTypeLimit(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	err := ms.CreateBucket("test")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 创建25种不同类型的文件（超过20个限制）
	contentTypes := []string{
		"text/plain", "text/html", "text/css", "text/javascript",
		"image/png", "image/jpeg", "image/gif", "image/webp",
		"application/json", "application/xml", "application/pdf", "application/zip",
		"video/mp4", "video/webm", "audio/mpeg", "audio/wav",
		"application/octet-stream", "text/csv", "text/markdown",
		"application/gzip", "application/x-tar",
		"image/svg+xml", "image/bmp", "image/tiff", "image/ico",
	}

	for i, ct := range contentTypes {
		ms.PutObject(&Object{
			Bucket:       "test",
			Key:          "file" + string(rune('a'+i)),
			Size:         int64((i + 1) * 1024), // 不同大小
			ETag:         "etag",
			ContentType:  ct,
			StoragePath:  "/path/file",
			LastModified: time.Now().UTC(),
		})
	}

	stats, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	// 应该只返回前20个类型
	if len(stats.TypeStats) > 20 {
		t.Errorf("类型统计应该限制在20个以内: got %d", len(stats.TypeStats))
	}
}

// TestGetExtensionFromContentType 测试 MIME 类型转扩展名
func TestGetExtensionFromContentType(t *testing.T) {
	testCases := []struct {
		contentType string
		expected    string
	}{
		{"image/png", "PNG"},
		{"image/jpeg", "JPEG"},
		{"text/plain", "TXT"},
		{"application/json", "JSON"},
		{"application/pdf", "PDF"},
		{"video/mp4", "MP4"},
		{"audio/mpeg", "MP3"},
		{"application/octet-stream", "Binary"},
		{"custom/unknown", "UNKNOWN"}, // 自定义类型
		{"text/x-custom", "X-CUSTOM"},  // 提取第二部分
		{"invalid", "Other"},           // 无效格式
	}

	for _, tc := range testCases {
		t.Run(tc.contentType, func(t *testing.T) {
			result := getExtensionFromContentType(tc.contentType)
			if result != tc.expected {
				t.Errorf("扩展名不匹配: got %s, want %s", result, tc.expected)
			}
		})
	}
}

// TestGetRecentObjects 测试获取最近对象
func TestGetRecentObjects(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	err := ms.CreateBucket("test")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 创建20个对象，时间递增
	now := time.Now().UTC()
	for i := 0; i < 20; i++ {
		ms.PutObject(&Object{
			Bucket:       "test",
			Key:          "file" + string(rune('a'+i)),
			Size:         1024,
			ETag:         "etag",
			ContentType:  "text/plain",
			StoragePath:  "/path/file",
			LastModified: now.Add(time.Duration(i) * time.Second),
		})
	}

	testCases := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{"默认限制", 0, 10},
		{"自定义10个", 10, 10},
		{"自定义5个", 5, 5},
		{"超过最大值", 100, 20}, // 最多50个，但我们只有20个对象
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objects, err := ms.GetRecentObjects(tc.limit)
			if err != nil {
				t.Fatalf("获取最近对象失败: %v", err)
			}

			if len(objects) != tc.expectedCount {
				t.Errorf("对象数量不匹配: got %d, want %d", len(objects), tc.expectedCount)
			}

			// 验证按时间倒序排列
			for i := 0; i < len(objects)-1; i++ {
				if objects[i].LastModified.Before(objects[i+1].LastModified) {
					t.Error("对象应该按时间倒序排列")
					break
				}
			}
		})
	}
}

// TestGetRecentObjectsLimit 测试最近对象限制验证
func TestGetRecentObjectsLimit(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	err := ms.CreateBucket("test")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 添加60个对象
	now := time.Now().UTC()
	for i := 0; i < 60; i++ {
		ms.PutObject(&Object{
			Bucket:       "test",
			Key:          "file" + string(rune(i)),
			Size:         1024,
			ETag:         "etag",
			ContentType:  "text/plain",
			StoragePath:  "/path/file",
			LastModified: now.Add(time.Duration(i) * time.Second),
		})
	}

	// 请求100个，应该限制在50个
	objects, err := ms.GetRecentObjects(100)
	if err != nil {
		t.Fatalf("获取最近对象失败: %v", err)
	}

	if len(objects) != 50 {
		t.Errorf("应该限制在50个: got %d", len(objects))
	}
}

// TestGetDiskUsage 测试磁盘使用统计
func TestGetDiskUsage(t *testing.T) {
	ms, fs, cleanup := setupStatsTest(t)
	defer cleanup()

	err := ms.CreateBucket("test")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 上传一些文件
	files := []struct {
		key  string
		size int64
	}{
		{"file1.txt", 1024},
		{"file2.txt", 2048},
		{"file3.txt", 4096},
	}

	for _, f := range files {
		content := strings.Repeat("x", int(f.size))
		_, _, err := fs.PutObject("test", f.key, strings.NewReader(content), f.size)
		if err != nil {
			t.Fatalf("上传文件失败: %v", err)
		}
	}

	// 获取磁盘使用
	totalSize, fileCount, err := fs.GetDiskUsage()
	if err != nil {
		t.Fatalf("获取磁盘使用失败: %v", err)
	}

	if fileCount != 3 {
		t.Errorf("文件数量错误: got %d, want 3", fileCount)
	}

	expectedSize := int64(1024 + 2048 + 4096)
	if totalSize != expectedSize {
		t.Errorf("总大小错误: got %d, want %d", totalSize, expectedSize)
	}
}

// TestGetDiskUsageEmpty 测试空文件存储
func TestGetDiskUsageEmpty(t *testing.T) {
	_, fs, cleanup := setupStatsTest(t)
	defer cleanup()

	totalSize, fileCount, err := fs.GetDiskUsage()
	if err != nil {
		t.Fatalf("获取磁盘使用失败: %v", err)
	}

	if fileCount != 0 {
		t.Errorf("空存储应该有0个文件: got %d", fileCount)
	}

	if totalSize != 0 {
		t.Errorf("空存储大小应该是0: got %d", totalSize)
	}
}

// TestGetStorageStatsWithEmptyContentType 测试空 ContentType 处理
func TestGetStorageStatsWithEmptyContentType(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	err := ms.CreateBucket("test")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 添加对象，一些有 ContentType，一些没有
	objects := []Object{
		{
			Bucket: "test", Key: "with_type.txt", Size: 1024,
			ETag: "e1", ContentType: "text/plain", StoragePath: "/path/1",
			LastModified: time.Now().UTC(),
		},
		{
			Bucket: "test", Key: "no_type.bin", Size: 2048,
			ETag: "e2", ContentType: "", StoragePath: "/path/2",
			LastModified: time.Now().UTC(),
		},
		{
			Bucket: "test", Key: "null_type.dat", Size: 4096,
			ETag: "e3", StoragePath: "/path/3",
			LastModified: time.Now().UTC(),
		},
	}

	for _, obj := range objects {
		if err := ms.PutObject(&obj); err != nil {
			t.Fatalf("添加对象失败: %v", err)
		}
	}

	stats, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	// 只有一个有效的 ContentType
	if len(stats.TypeStats) != 1 {
		t.Errorf("类型统计应该只包含有效类型: got %d, want 1", len(stats.TypeStats))
	}

	if stats.TypeStats[0].ContentType != "text/plain" {
		t.Errorf("类型错误: got %s, want text/plain", stats.TypeStats[0].ContentType)
	}
}

// TestStorageStatsConsistency 测试统计数据一致性
func TestStorageStatsConsistency(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	err := ms.CreateBucket("test")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 添加对象
	ms.PutObject(&Object{
		Bucket: "test", Key: "file.txt", Size: 1024,
		ETag: "e1", ContentType: "text/plain", StoragePath: "/path/1",
		LastModified: time.Now().UTC(),
	})

	stats1, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("第一次获取统计失败: %v", err)
	}

	// 再次获取，应该一致
	stats2, err := ms.GetStorageStats()
	if err != nil {
		t.Fatalf("第二次获取统计失败: %v", err)
	}

	if stats1.TotalObjects != stats2.TotalObjects {
		t.Error("统计数据应该保持一致")
	}

	if stats1.TotalSize != stats2.TotalSize {
		t.Error("统计数据应该保持一致")
	}
}

// TestGetRecentObjectsFields 测试最近对象字段完整性
func TestGetRecentObjectsFields(t *testing.T) {
	ms, _, cleanup := setupStatsTest(t)
	defer cleanup()

	err := ms.CreateBucket("test")
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	testObj := Object{
		Bucket:       "test",
		Key:          "test.txt",
		Size:         1024,
		ETag:         "test-etag",
		ContentType:  "text/plain",
		StoragePath:  "/test/path",
		LastModified: time.Now().UTC(),
	}

	if err := ms.PutObject(&testObj); err != nil {
		t.Fatalf("添加对象失败: %v", err)
	}

	objects, err := ms.GetRecentObjects(1)
	if err != nil {
		t.Fatalf("获取最近对象失败: %v", err)
	}

	if len(objects) != 1 {
		t.Fatal("应该返回1个对象")
	}

	obj := objects[0]
	if obj.Bucket != testObj.Bucket {
		t.Errorf("Bucket不匹配: got %s, want %s", obj.Bucket, testObj.Bucket)
	}
	if obj.Key != testObj.Key {
		t.Errorf("Key不匹配: got %s, want %s", obj.Key, testObj.Key)
	}
	if obj.Size != testObj.Size {
		t.Errorf("Size不匹配: got %d, want %d", obj.Size, testObj.Size)
	}
	if obj.ETag != testObj.ETag {
		t.Errorf("ETag不匹配: got %s, want %s", obj.ETag, testObj.ETag)
	}
	if obj.ContentType != testObj.ContentType {
		t.Errorf("ContentType不匹配: got %s, want %s", obj.ContentType, testObj.ContentType)
	}
	if obj.StoragePath != testObj.StoragePath {
		t.Errorf("StoragePath不匹配: got %s, want %s", obj.StoragePath, testObj.StoragePath)
	}
}

// BenchmarkGetStorageStats 存储统计性能基准测试
func BenchmarkGetStorageStats(b *testing.B) {
	ms, cleanup := setupMetadataStore(&testing.T{})
	defer cleanup()

	// 准备测试数据
	ms.CreateBucket("test")
	for i := 0; i < 100; i++ {
		ms.PutObject(&Object{
			Bucket:       "test",
			Key:          "file" + string(rune(i)),
			Size:         int64(i * 1024),
			ETag:         "etag",
			ContentType:  "text/plain",
			StoragePath:  "/path/file",
			LastModified: time.Now().UTC(),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.GetStorageStats()
	}
}

// BenchmarkGetRecentObjects 获取最近对象性能基准测试
func BenchmarkGetRecentObjects(b *testing.B) {
	ms, cleanup := setupMetadataStore(&testing.T{})
	defer cleanup()

	// 准备测试数据
	ms.CreateBucket("test")
	now := time.Now().UTC()
	for i := 0; i < 100; i++ {
		ms.PutObject(&Object{
			Bucket:       "test",
			Key:          "file" + string(rune(i)),
			Size:         1024,
			ETag:         "etag",
			ContentType:  "text/plain",
			StoragePath:  "/path/file",
			LastModified: now.Add(time.Duration(i) * time.Second),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.GetRecentObjects(10)
	}
}
