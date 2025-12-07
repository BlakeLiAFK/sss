package storage

import (
	"sync"
	"testing"
	"time"
)

// setupMigrateManager 为迁移测试创建管理器
func setupMigrateManager(t *testing.T) (*MigrateManager, *MetadataStore, func()) {
	t.Helper()

	// 创建元数据存储
	store, cleanup := setupMetadataStore(t)

	// 创建文件存储
	fileStore, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 重置单例（测试需要）
	migrateOnce = sync.Once{}
	migrateManager = nil

	// 获取管理器
	manager := GetMigrateManager(store, fileStore)

	return manager, store, cleanup
}

// TestGetMigrateManagerSingleton 测试单例模式
func TestGetMigrateManagerSingleton(t *testing.T) {
	manager1, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建第二个文件存储
	fileStore2, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 再次调用应该返回同一个实例
	manager2 := GetMigrateManager(store, fileStore2)

	if manager1 != manager2 {
		t.Error("GetMigrateManager应该返回同一个实例")
	}
}

// TestGenerateJobID 测试任务ID生成
func TestGenerateJobID(t *testing.T) {
	// 生成多个ID
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateJobID()

		// 检查格式（16字节hex编码 = 32字符）
		if len(id) != 32 {
			t.Errorf("任务ID长度错误: got %d, want 32", len(id))
		}

		// 检查唯一性
		if ids[id] {
			t.Errorf("生成了重复的任务ID: %s", id)
		}
		ids[id] = true
	}
}

// TestStartMigrationValidation 测试配置验证
func TestStartMigrationValidation(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	testCases := []struct {
		name      string
		config    MigrateConfig
		expectErr string
	}{
		{
			name:      "缺少源端点",
			config:    MigrateConfig{},
			expectErr: "sourceEndpoint is required",
		},
		{
			name: "缺少源凭证",
			config: MigrateConfig{
				SourceEndpoint: "http://s3.example.com",
			},
			expectErr: "source credentials are required",
		},
		{
			name: "缺少源桶名",
			config: MigrateConfig{
				SourceEndpoint:  "http://s3.example.com",
				SourceAccessKey: "access",
				SourceSecretKey: "secret",
			},
			expectErr: "sourceBucket is required",
		},
		{
			name: "缺少目标桶名",
			config: MigrateConfig{
				SourceEndpoint:  "http://s3.example.com",
				SourceAccessKey: "access",
				SourceSecretKey: "secret",
				SourceBucket:    "source",
			},
			expectErr: "targetBucket is required",
		},
		{
			name: "目标桶不存在",
			config: MigrateConfig{
				SourceEndpoint:  "http://s3.example.com",
				SourceAccessKey: "access",
				SourceSecretKey: "secret",
				SourceBucket:    "source",
				TargetBucket:    "nonexistent",
			},
			expectErr: "target bucket not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := manager.StartMigration(tc.config)
			if err == nil {
				t.Error("期望返回错误，但没有")
			} else if tc.expectErr != "" && err.Error()[:len(tc.expectErr)] != tc.expectErr {
				t.Errorf("错误信息不匹配: got %q, want contains %q", err.Error(), tc.expectErr)
			}
		})
	}
}

// TestStartMigrationSuccess 测试成功启动迁移
func TestStartMigrationSuccess(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	// 验证任务ID
	if len(jobID) != 32 {
		t.Errorf("任务ID长度错误: got %d, want 32", len(jobID))
	}

	// 等待后台 goroutine 启动并稳定状态
	time.Sleep(50 * time.Millisecond)

	// 验证任务已创建
	progress := manager.GetProgress(jobID)
	if progress == nil {
		t.Fatal("任务进度为空")
	}

	if progress.JobID != jobID {
		t.Errorf("任务ID不匹配: got %s, want %s", progress.JobID, jobID)
	}

	// 状态应该是 pending 或 running（goroutine可能已经启动）
	if progress.Status != "pending" && progress.Status != "running" && progress.Status != "failed" {
		t.Errorf("任务状态错误: got %s", progress.Status)
	}

	// 配置应该保存
	if progress.Config.SourceBucket != "source" {
		t.Errorf("配置未保存: got %s, want source", progress.Config.SourceBucket)
	}
}

// TestGetProgress 测试获取进度
func TestGetProgress(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	// 获取进度
	progress := manager.GetProgress(jobID)
	if progress == nil {
		t.Fatal("进度为空")
	}

	if progress.JobID != jobID {
		t.Errorf("任务ID不匹配: got %s, want %s", progress.JobID, jobID)
	}

	// 获取不存在的任务
	nonexistent := manager.GetProgress("nonexistent")
	if nonexistent != nil {
		t.Error("不存在的任务应该返回nil")
	}
}

// TestGetAllJobs 测试获取所有任务
func TestGetAllJobs(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	// 初始应该为空
	jobs := manager.GetAllJobs()
	if len(jobs) != 0 {
		t.Errorf("初始任务数量应该为0: got %d", len(jobs))
	}

	// 启动3个任务
	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		jobID, err := manager.StartMigration(cfg)
		if err != nil {
			t.Fatalf("启动迁移%d失败: %v", i, err)
		}
		jobIDs[i] = jobID
	}

	// 获取所有任务
	jobs = manager.GetAllJobs()
	if len(jobs) != 3 {
		t.Errorf("任务数量不匹配: got %d, want 3", len(jobs))
	}

	// 验证所有任务ID都在列表中
	foundIDs := make(map[string]bool)
	for _, job := range jobs {
		foundIDs[job.JobID] = true
	}

	for _, id := range jobIDs {
		if !foundIDs[id] {
			t.Errorf("任务ID %s 未在列表中", id)
		}
	}
}

// TestCancelMigration 测试取消迁移
func TestCancelMigration(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	// 等待任务启动或失败
	time.Sleep(100 * time.Millisecond)

	// 取消任务
	err = manager.CancelMigration(jobID)
	if err != nil {
		t.Fatalf("取消任务失败: %v", err)
	}

	// 验证状态
	progress := manager.GetProgress(jobID)
	if progress == nil {
		t.Fatal("进度为空")
	}

	if progress.Status != "cancelled" {
		t.Errorf("任务状态应该是cancelled: got %s", progress.Status)
	}

	if progress.EndTime == nil {
		t.Error("结束时间应该被设置")
	}

	// 再次取消应该返回错误
	err = manager.CancelMigration(jobID)
	if err == nil {
		t.Error("重复取消应该返回错误")
	}

	// 取消不存在的任务
	err = manager.CancelMigration("nonexistent")
	if err == nil {
		t.Error("取消不存在的任务应该返回错误")
	}
}

// TestDeleteJob 测试删除任务
func TestDeleteJob(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	// 等待任务启动或失败
	time.Sleep(100 * time.Millisecond)

	// 尝试删除运行中的任务（应该失败）
	progress := manager.GetProgress(jobID)
	if progress != nil && (progress.Status == "running" || progress.Status == "pending") {
		err = manager.DeleteJob(jobID)
		if err == nil {
			t.Error("删除运行中的任务应该返回错误")
		}
	}

	// 先取消任务
	_ = manager.CancelMigration(jobID)

	// 删除已取消的任务
	err = manager.DeleteJob(jobID)
	if err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}

	// 验证任务已删除
	progress = manager.GetProgress(jobID)
	if progress != nil {
		t.Error("任务应该已被删除")
	}

	// 删除不存在的任务
	err = manager.DeleteJob("nonexistent")
	if err == nil {
		t.Error("删除不存在的任务应该返回错误")
	}
}

// TestGetJobStats 测试任务统计
func TestGetJobStats(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	// 初始统计
	stats := manager.GetJobStats()
	if stats["total"] != 0 {
		t.Errorf("初始total应该为0: got %d", stats["total"])
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	// 启动2个任务
	jobID1, _ := manager.StartMigration(cfg)
	jobID2, _ := manager.StartMigration(cfg)

	// 等待任务状态稳定
	time.Sleep(200 * time.Millisecond)

	// 取消一个任务
	_ = manager.CancelMigration(jobID1)

	// 等待状态更新
	time.Sleep(50 * time.Millisecond)

	// 获取统计
	stats = manager.GetJobStats()

	if stats["total"] != 2 {
		t.Errorf("总任务数应该为2: got %d", stats["total"])
	}

	if stats["cancelled"] < 1 {
		t.Errorf("已取消任务数至少为1: got %d", stats["cancelled"])
	}

	// 删除已取消的任务
	_ = manager.DeleteJob(jobID1)

	stats = manager.GetJobStats()
	if stats["total"] != 1 {
		t.Errorf("删除后总任务数应该为1: got %d", stats["total"])
	}

	// 取消第二个任务以便清理
	_ = manager.CancelMigration(jobID2)
}

// TestMigrateConfigDefaultRegion 测试默认区域设置
func TestMigrateConfigDefaultRegion(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
		// 不设置 SourceRegion
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	// 验证配置中的区域被设置为默认值
	progress := manager.GetProgress(jobID)
	if progress == nil {
		t.Fatal("进度为空")
	}

	if progress.Config.SourceRegion != "us-east-1" {
		t.Errorf("默认区域应该是us-east-1: got %s", progress.Config.SourceRegion)
	}

	// 清理
	time.Sleep(100 * time.Millisecond)
	_ = manager.CancelMigration(jobID)
}

// TestMigrateConfigWithPrefix 测试前缀配置
func TestMigrateConfigWithPrefix(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		SourcePrefix:    "backup/",
		TargetBucket:    "target",
		TargetPrefix:    "restored/",
		OverwriteExist:  true,
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	// 验证配置保存
	progress := manager.GetProgress(jobID)
	if progress == nil {
		t.Fatal("进度为空")
	}

	if progress.Config.SourcePrefix != "backup/" {
		t.Errorf("源前缀不匹配: got %s, want backup/", progress.Config.SourcePrefix)
	}

	if progress.Config.TargetPrefix != "restored/" {
		t.Errorf("目标前缀不匹配: got %s, want restored/", progress.Config.TargetPrefix)
	}

	if !progress.Config.OverwriteExist {
		t.Error("覆盖选项应该为true")
	}

	// 清理
	time.Sleep(100 * time.Millisecond)
	_ = manager.CancelMigration(jobID)
}

// TestConcurrentMigration 测试并发迁移任务
func TestConcurrentMigration(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	const numJobs = 10
	var wg sync.WaitGroup
	jobIDs := make(chan string, numJobs)
	errors := make(chan error, numJobs)

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	// 并发启动多个任务
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jobID, err := manager.StartMigration(cfg)
			if err != nil {
				errors <- err
				return
			}
			jobIDs <- jobID
		}()
	}

	wg.Wait()
	close(jobIDs)
	close(errors)

	// 检查错误
	for err := range errors {
		t.Errorf("并发启动任务失败: %v", err)
	}

	// 检查所有任务ID唯一
	uniqueIDs := make(map[string]bool)
	var ids []string
	for id := range jobIDs {
		if uniqueIDs[id] {
			t.Errorf("生成了重复的任务ID: %s", id)
		}
		uniqueIDs[id] = true
		ids = append(ids, id)
	}

	if len(uniqueIDs) != numJobs {
		t.Errorf("任务数量不匹配: got %d, want %d", len(uniqueIDs), numJobs)
	}

	// 验证所有任务都在管理器中
	allJobs := manager.GetAllJobs()
	if len(allJobs) != numJobs {
		t.Errorf("管理器中的任务数量不匹配: got %d, want %d", len(allJobs), numJobs)
	}

	// 清理：取消所有任务
	time.Sleep(200 * time.Millisecond)
	for _, id := range ids {
		_ = manager.CancelMigration(id)
	}
}

// TestMigrateProgressFields 测试进度字段
func TestMigrateProgressFields(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	progress := manager.GetProgress(jobID)
	if progress == nil {
		t.Fatal("进度为空")
	}

	// 验证初始字段
	if progress.JobID == "" {
		t.Error("JobID不应该为空")
	}

	if progress.StartTime.IsZero() {
		t.Error("StartTime应该被设置")
	}

	if progress.TotalObjects != 0 {
		t.Error("TotalObjects初始应该为0")
	}

	if progress.Completed != 0 {
		t.Error("Completed初始应该为0")
	}

	if progress.Failed != 0 {
		t.Error("Failed初始应该为0")
	}

	if progress.Skipped != 0 {
		t.Error("Skipped初始应该为0")
	}

	if progress.TotalSize != 0 {
		t.Error("TotalSize初始应该为0")
	}

	if progress.TransferSize != 0 {
		t.Error("TransferSize初始应该为0")
	}

	// 清理
	time.Sleep(100 * time.Millisecond)
	_ = manager.CancelMigration(jobID)
}

// TestMigrateStatusTransitions 测试状态转换
func TestMigrateStatusTransitions(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()

	// 创建目标桶
	err := store.CreateBucket("target")
	if err != nil {
		t.Fatalf("创建目标桶失败: %v", err)
	}

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobID, err := manager.StartMigration(cfg)
	if err != nil {
		t.Fatalf("启动迁移失败: %v", err)
	}

	// 等待后台 goroutine 启动并更新状态
	time.Sleep(50 * time.Millisecond)

	// 状态转换：pending -> running/failed
	progress := manager.GetProgress(jobID)
	if progress.Status != "pending" && progress.Status != "running" && progress.Status != "failed" {
		t.Errorf("初始状态应该是pending/running/failed之一: got %s", progress.Status)
	}

	// 等待状态稳定
	time.Sleep(200 * time.Millisecond)

	// 取消任务：-> cancelled
	err = manager.CancelMigration(jobID)
	if err != nil {
		t.Fatalf("取消任务失败: %v", err)
	}

	progress = manager.GetProgress(jobID)
	if progress.Status != "cancelled" {
		t.Errorf("取消后状态应该是cancelled: got %s", progress.Status)
	}

	// EndTime应该被设置
	if progress.EndTime == nil {
		t.Error("取消后EndTime应该被设置")
	}
}

// BenchmarkStartMigration 启动迁移性能测试
func BenchmarkStartMigration(b *testing.B) {
	manager, store, cleanup := setupMigrateManager(&testing.T{})
	defer cleanup()

	// 创建目标桶
	_ = store.CreateBucket("target")

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobID, err := manager.StartMigration(cfg)
		if err != nil {
			b.Fatalf("启动迁移失败: %v", err)
		}
		// 立即取消以避免实际执行
		time.Sleep(10 * time.Millisecond)
		_ = manager.CancelMigration(jobID)
	}
}

// BenchmarkGetProgress 获取进度性能测试
func BenchmarkGetProgress(b *testing.B) {
	manager, store, cleanup := setupMigrateManager(&testing.T{})
	defer cleanup()

	// 创建目标桶
	_ = store.CreateBucket("target")

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	jobID, _ := manager.StartMigration(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetProgress(jobID)
	}

	// 清理
	time.Sleep(100 * time.Millisecond)
	_ = manager.CancelMigration(jobID)
}

// BenchmarkGetJobStats 获取统计性能测试
func BenchmarkGetJobStats(b *testing.B) {
	manager, store, cleanup := setupMigrateManager(&testing.T{})
	defer cleanup()

	// 创建目标桶
	_ = store.CreateBucket("target")

	cfg := MigrateConfig{
		SourceEndpoint:  "http://localhost:9000",
		SourceAccessKey: "minioadmin",
		SourceSecretKey: "minioadmin",
		SourceBucket:    "source",
		TargetBucket:    "target",
	}

	// 创建多个任务
	jobIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		jobID, _ := manager.StartMigration(cfg)
		jobIDs[i] = jobID
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetJobStats()
	}

	// 清理
	time.Sleep(200 * time.Millisecond)
	for _, jobID := range jobIDs {
		_ = manager.CancelMigration(jobID)
	}
}
