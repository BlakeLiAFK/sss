package storage

import (
	"strings"
	"testing"
	"time"
)

// setupAuditTest 为审计测试创建MetadataStore
func setupAuditTest(t *testing.T) (*MetadataStore, func()) {
	t.Helper()
	return setupMetadataStore(t)
}

// TestWriteAuditLog 测试写入审计日志
func TestWriteAuditLog(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	testCases := []struct {
		name string
		log  *AuditLog
	}{
		{
			name: "登录成功日志",
			log: &AuditLog{
				Action:    AuditActionLogin,
				Actor:     "admin",
				IP:        "192.168.1.100",
				Resource:  "",
				Detail:    "{\"method\":\"password\"}",
				Success:   true,
				UserAgent: "Mozilla/5.0",
			},
		},
		{
			name: "登录失败日志",
			log: &AuditLog{
				Action:    AuditActionLoginFailed,
				Actor:     "hacker",
				IP:        "1.2.3.4",
				Resource:  "",
				Detail:    "{\"reason\":\"invalid_password\"}",
				Success:   false,
				UserAgent: "curl/7.68.0",
			},
		},
		{
			name: "创建桶日志",
			log: &AuditLog{
				Action:    AuditActionBucketCreate,
				Actor:     "admin",
				IP:        "192.168.1.100",
				Resource:  "my-bucket",
				Detail:    "{\"region\":\"default\"}",
				Success:   true,
				UserAgent: "aws-cli/2.0",
			},
		},
		{
			name: "上传对象日志",
			log: &AuditLog{
				Timestamp: time.Now().UTC(), // 手动设置时间戳
				Action:    AuditActionObjectUpload,
				Actor:     "apikey_123456",
				IP:        "10.0.0.5",
				Resource:  "my-bucket/file.txt",
				Detail:    "{\"size\":1024,\"content_type\":\"text/plain\"}",
				Success:   true,
				UserAgent: "python-requests/2.25.1",
			},
		},
		{
			name: "空字段日志",
			log: &AuditLog{
				Action:  AuditActionSystemInstall,
				Success: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ms.WriteAuditLog(tc.log)
			if err != nil {
				t.Fatalf("写入审计日志失败: %v", err)
			}

			// 验证时间戳已设置
			if tc.log.Timestamp.IsZero() {
				t.Error("时间戳应该被自动设置")
			}
		})
	}
}

// TestQueryAuditLogs 测试查询审计日志
func TestQueryAuditLogs(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	// 准备测试数据
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	testLogs := []*AuditLog{
		{
			Timestamp: yesterday,
			Action:    AuditActionLogin,
			Actor:     "admin",
			IP:        "192.168.1.100",
			Resource:  "",
			Success:   true,
			UserAgent: "browser",
		},
		{
			Timestamp: now,
			Action:    AuditActionBucketCreate,
			Actor:     "admin",
			IP:        "192.168.1.100",
			Resource:  "bucket1",
			Success:   true,
			UserAgent: "aws-cli",
		},
		{
			Timestamp: now.Add(time.Minute),
			Action:    AuditActionObjectUpload,
			Actor:     "user1",
			IP:        "10.0.0.5",
			Resource:  "bucket1/file.txt",
			Success:   true,
			UserAgent: "sdk",
		},
		{
			Timestamp: now.Add(2 * time.Minute),
			Action:    AuditActionLoginFailed,
			Actor:     "hacker",
			IP:        "1.2.3.4",
			Resource:  "",
			Success:   false,
			UserAgent: "curl",
		},
	}

	for _, log := range testLogs {
		if err := ms.WriteAuditLog(log); err != nil {
			t.Fatalf("写入测试数据失败: %v", err)
		}
	}

	testCases := []struct {
		name          string
		query         *AuditLogQuery
		expectedCount int
		checkFunc     func(t *testing.T, logs []AuditLog)
	}{
		{
			name:          "查询所有日志",
			query:         &AuditLogQuery{Limit: 100},
			expectedCount: 4,
		},
		{
			name: "按操作类型过滤",
			query: &AuditLogQuery{
				Action: AuditActionLogin,
				Limit:  100,
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, logs []AuditLog) {
				if logs[0].Action != AuditActionLogin {
					t.Errorf("操作类型不匹配: got %s", logs[0].Action)
				}
			},
		},
		{
			name: "按操作者过滤（完全匹配）",
			query: &AuditLogQuery{
				Actor: "admin",
				Limit: 100,
			},
			expectedCount: 2,
		},
		{
			name: "按操作者过滤（部分匹配）",
			query: &AuditLogQuery{
				Actor: "user",
				Limit: 100,
			},
			expectedCount: 1,
		},
		{
			name: "按IP过滤",
			query: &AuditLogQuery{
				IP:    "192.168",
				Limit: 100,
			},
			expectedCount: 2,
		},
		{
			name: "按资源过滤",
			query: &AuditLogQuery{
				Resource: "bucket1",
				Limit:    100,
			},
			expectedCount: 2,
		},
		{
			name: "按成功状态过滤",
			query: &AuditLogQuery{
				Success: boolPtr(false),
				Limit:   100,
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, logs []AuditLog) {
				if logs[0].Success {
					t.Error("应该只返回失败的日志")
				}
			},
		},
		{
			name: "按时间范围过滤",
			query: &AuditLogQuery{
				StartTime: &yesterday,
				EndTime:   timePtr(now.Add(30 * time.Second)),
				Limit:     100,
			},
			expectedCount: 2, // yesterday 和 now 的日志
		},
		{
			name: "分页查询",
			query: &AuditLogQuery{
				Limit:  2,
				Offset: 0,
			},
			expectedCount: 2,
		},
		{
			name: "分页查询第二页",
			query: &AuditLogQuery{
				Limit:  2,
				Offset: 2,
			},
			expectedCount: 2,
		},
		{
			name: "组合条件查询",
			query: &AuditLogQuery{
				Actor:   "admin",
				Success: boolPtr(true),
				Limit:   100,
			},
			expectedCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs, total, err := ms.QueryAuditLogs(tc.query)
			if err != nil {
				t.Fatalf("查询审计日志失败: %v", err)
			}

			if len(logs) != tc.expectedCount {
				t.Errorf("日志数量不匹配: got %d, want %d", len(logs), tc.expectedCount)
			}

			// 验证总数
			if total < tc.expectedCount {
				t.Errorf("总数应该 >= 返回数量: total=%d, returned=%d", total, tc.expectedCount)
			}

			// 验证排序（应该按时间倒序）
			for i := 0; i < len(logs)-1; i++ {
				if logs[i].Timestamp.Before(logs[i+1].Timestamp) {
					t.Error("日志应该按时间倒序排列")
				}
			}

			if tc.checkFunc != nil {
				tc.checkFunc(t, logs)
			}
		})
	}
}

// TestQueryAuditLogsLimitValidation 测试查询限制验证
func TestQueryAuditLogsLimitValidation(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	// 写入一些测试数据
	for i := 0; i < 10; i++ {
		ms.WriteAuditLog(&AuditLog{
			Action:  AuditActionLogin,
			Success: true,
		})
	}

	testCases := []struct {
		name          string
		limit         int
		expectedLimit int
	}{
		{"默认限制", 0, 100},
		{"小于最大值", 50, 50},
		{"超过最大值", 2000, 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs, _, err := ms.QueryAuditLogs(&AuditLogQuery{Limit: tc.limit})
			if err != nil {
				t.Fatalf("查询失败: %v", err)
			}

			if len(logs) > tc.expectedLimit {
				t.Errorf("返回数量超过限制: got %d, max %d", len(logs), tc.expectedLimit)
			}
		})
	}
}

// TestGetRecentAuditLogs 测试获取最近的审计日志
func TestGetRecentAuditLogs(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	// 写入测试数据
	now := time.Now().UTC()
	for i := 0; i < 100; i++ {
		ms.WriteAuditLog(&AuditLog{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Action:    AuditActionLogin,
			Success:   true,
		})
	}

	testCases := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{"默认限制", 0, 50},
		{"自定义限制", 10, 10},
		{"超过总数", 200, 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := ms.GetRecentAuditLogs(tc.limit)
			if err != nil {
				t.Fatalf("获取最近日志失败: %v", err)
			}

			if len(logs) != tc.expectedCount {
				t.Errorf("日志数量不匹配: got %d, want %d", len(logs), tc.expectedCount)
			}

			// 验证按时间倒序
			for i := 0; i < len(logs)-1; i++ {
				if logs[i].Timestamp.Before(logs[i+1].Timestamp) {
					t.Error("日志应该按时间倒序")
				}
			}
		})
	}
}

// TestCleanOldAuditLogs 测试清理旧的审计日志
func TestCleanOldAuditLogs(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	now := time.Now().UTC()
	testCases := []struct {
		timestamp time.Time
	}{
		{now.AddDate(0, 0, -100)}, // 100天前
		{now.AddDate(0, 0, -95)},  // 95天前
		{now.AddDate(0, 0, -91)},  // 91天前
		{now.AddDate(0, 0, -89)},  // 89天前
		{now.AddDate(0, 0, -30)},  // 30天前
		{now.AddDate(0, 0, -1)},   // 1天前
		{now},                     // 今天
	}

	for _, tc := range testCases {
		ms.WriteAuditLog(&AuditLog{
			Timestamp: tc.timestamp,
			Action:    AuditActionLogin,
			Success:   true,
		})
	}

	// 清理 90 天前的日志
	deleted, err := ms.CleanOldAuditLogs(90)
	if err != nil {
		t.Fatalf("清理旧日志失败: %v", err)
	}

	if deleted != 3 { // 100, 95, 91 天前的应该被删除
		t.Errorf("删除数量不匹配: got %d, want 3", deleted)
	}

	// 验证剩余日志
	logs, total, err := ms.QueryAuditLogs(&AuditLogQuery{Limit: 100})
	if err != nil {
		t.Fatalf("查询日志失败: %v", err)
	}

	if total != 4 { // 89, 30, 1 天前和今天
		t.Errorf("剩余日志数量不匹配: got %d, want 4", total)
	}

	if len(logs) != 4 {
		t.Errorf("返回日志数量不匹配: got %d, want 4", len(logs))
	}
}

// TestCleanOldAuditLogsDefaultRetention 测试默认保留期
func TestCleanOldAuditLogsDefaultRetention(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	now := time.Now().UTC()
	ms.WriteAuditLog(&AuditLog{
		Timestamp: now.AddDate(0, 0, -100),
		Action:    AuditActionLogin,
		Success:   true,
	})
	ms.WriteAuditLog(&AuditLog{
		Timestamp: now.AddDate(0, 0, -50),
		Action:    AuditActionLogin,
		Success:   true,
	})

	// 使用默认保留期（90天）
	deleted, err := ms.CleanOldAuditLogs(0)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}

	if deleted != 1 { // 只有 100 天前的应该被删除
		t.Errorf("删除数量不匹配: got %d, want 1", deleted)
	}
}

// TestGetAuditStats 测试获取审计统计
func TestGetAuditStats(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	// 使用本地时间（与 GetAuditStats 一致）
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	// 准备测试数据
	testLogs := []*AuditLog{
		// 昨天的日志
		{Timestamp: yesterday, Action: AuditActionLogin, Success: true},
		{Timestamp: yesterday, Action: AuditActionLogin, Success: false},
		// 今天的日志
		{Timestamp: today.Add(time.Hour), Action: AuditActionLogin, Success: true},
		{Timestamp: today.Add(2 * time.Hour), Action: AuditActionBucketCreate, Success: true},
		{Timestamp: today.Add(3 * time.Hour), Action: AuditActionBucketCreate, Success: true},
		{Timestamp: today.Add(4 * time.Hour), Action: AuditActionObjectUpload, Success: true},
		{Timestamp: today.Add(5 * time.Hour), Action: AuditActionLoginFailed, Success: false},
	}

	for _, log := range testLogs {
		if err := ms.WriteAuditLog(log); err != nil {
			t.Fatalf("写入测试数据失败: %v", err)
		}
	}

	stats, err := ms.GetAuditStats()
	if err != nil {
		t.Fatalf("获取统计信息失败: %v", err)
	}

	// 验证总数
	if total, ok := stats["total"].(int); !ok || total != 7 {
		t.Errorf("总数不匹配: got %v, want 7", stats["total"])
	}

	// 验证今日数量（应该有5条今天的日志）
	todayCount, ok := stats["today"].(int)
	if !ok {
		t.Fatal("today 字段类型错误")
	}
	if todayCount != 5 {
		t.Errorf("今日数量不匹配: got %d, want 5", todayCount)
	}

	// 验证失败数量
	if failedCount, ok := stats["failed"].(int); !ok || failedCount != 2 {
		t.Errorf("失败数量不匹配: got %v, want 2", stats["failed"])
	}

	// 验证按操作类型统计（注意：GetAuditStats 只返回前10个最多的操作类型）
	actionStats, ok := stats["by_action"].(map[string]int)
	if !ok {
		t.Fatal("by_action 类型错误")
	}

	// 验证我们期望的操作类型至少出现了
	minExpectedActions := map[string]int{
		string(AuditActionLogin):        2, // 昨天1个失败 + 昨天1个成功 + 今天1个成功 = 3，但只统计login成功2次
		string(AuditActionBucketCreate): 2,
		string(AuditActionObjectUpload): 1,
		string(AuditActionLoginFailed):  1,
	}

	for action, minCount := range minExpectedActions {
		if count, ok := actionStats[action]; !ok || count < minCount {
			t.Errorf("操作类型 %s 统计应该至少有 %d 次: got %d", action, minCount, count)
		}
	}
}

// TestGetAuditStatsEmpty 测试空数据统计
func TestGetAuditStatsEmpty(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	stats, err := ms.GetAuditStats()
	if err != nil {
		t.Fatalf("获取空统计失败: %v", err)
	}

	if total, ok := stats["total"].(int); !ok || total != 0 {
		t.Errorf("空数据总数应该为0: got %v", stats["total"])
	}

	if todayCount, ok := stats["today"].(int); !ok || todayCount != 0 {
		t.Errorf("空数据今日数量应该为0: got %v", stats["today"])
	}

	if failedCount, ok := stats["failed"].(int); !ok || failedCount != 0 {
		t.Errorf("空数据失败数量应该为0: got %v", stats["failed"])
	}
}

// TestAuditLogActions 测试所有操作类型常量
func TestAuditLogActions(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	actions := []AuditAction{
		// 认证相关
		AuditActionLogin,
		AuditActionLoginFailed,
		AuditActionLogout,
		AuditActionPasswordReset,
		// 系统相关
		AuditActionSystemInstall,
		AuditActionSettingsUpdate,
		AuditActionPasswordChange,
		// Bucket 相关
		AuditActionBucketCreate,
		AuditActionBucketDelete,
		AuditActionBucketSetPublic,
		AuditActionBucketSetPrivate,
		// 对象相关
		AuditActionObjectUpload,
		AuditActionObjectDelete,
		AuditActionObjectCopy,
		AuditActionBatchDelete,
		// API Key 相关
		AuditActionAPIKeyCreate,
		AuditActionAPIKeyDelete,
		AuditActionAPIKeyResetSecret,
		AuditActionAPIKeyUpdate,
		AuditActionAPIKeySetPerm,
		AuditActionAPIKeyDelPerm,
		// 迁移相关
		AuditActionMigrateCreate,
		AuditActionMigrateCancel,
	}

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			err := ms.WriteAuditLog(&AuditLog{
				Action:  action,
				Success: true,
			})
			if err != nil {
				t.Errorf("写入 %s 操作日志失败: %v", action, err)
			}
		})
	}

	// 验证所有操作都已记录
	logs, total, err := ms.QueryAuditLogs(&AuditLogQuery{Limit: 100})
	if err != nil {
		t.Fatalf("查询日志失败: %v", err)
	}

	if total != len(actions) {
		t.Errorf("记录的操作数量不匹配: got %d, want %d", total, len(actions))
	}

	if len(logs) != len(actions) {
		t.Errorf("返回的日志数量不匹配: got %d, want %d", len(logs), len(actions))
	}
}

// TestAuditLogConcurrentWrites 测试并发写入审计日志
func TestAuditLogConcurrentWrites(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	const numGoroutines = 10
	const logsPerGoroutine = 10

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*logsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			for j := 0; j < logsPerGoroutine; j++ {
				err := ms.WriteAuditLog(&AuditLog{
					Action:  AuditActionLogin,
					Actor:   "concurrent_test",
					Success: true,
				})
				if err != nil {
					errors <- err
				}
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 检查错误
	close(errors)
	for err := range errors {
		t.Errorf("并发写入出错: %v", err)
	}

	// 验证写入的总数
	logs, total, err := ms.QueryAuditLogs(&AuditLogQuery{Actor: "concurrent_test", Limit: 200})
	if err != nil {
		t.Fatalf("查询日志失败: %v", err)
	}

	expectedTotal := numGoroutines * logsPerGoroutine
	if total != expectedTotal {
		t.Errorf("并发写入总数不匹配: got %d, want %d", total, expectedTotal)
	}

	if len(logs) != expectedTotal {
		t.Errorf("返回日志数量不匹配: got %d, want %d", len(logs), expectedTotal)
	}
}

// TestAuditLogSpecialCharacters 测试特殊字符处理
func TestAuditLogSpecialCharacters(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	testCases := []struct {
		name   string
		actor  string
		detail string
	}{
		{"中文字符", "管理员", "{\"操作\":\"登录\"}"},
		{"SQL注入尝试", "admin' OR '1'='1", "'; DROP TABLE audit_logs; --"},
		{"特殊符号", "user@domain.com", "{\"key\":\"value with 'quotes' and \\\"escapes\\\"\"}"},
		{"长文本", strings.Repeat("a", 1000), strings.Repeat("b", 5000)},
		{"Unicode", "用户👤", "{\"emoji\":\"🔐🔑\"}"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ms.WriteAuditLog(&AuditLog{
				Action:  AuditActionLogin,
				Actor:   tc.actor,
				Detail:  tc.detail,
				Success: true,
			})
			if err != nil {
				t.Fatalf("写入特殊字符日志失败: %v", err)
			}

			// 查询并验证
			logs, _, err := ms.QueryAuditLogs(&AuditLogQuery{
				Actor: tc.actor,
				Limit: 1,
			})
			if err != nil {
				t.Fatalf("查询失败: %v", err)
			}

			if len(logs) != 1 {
				t.Fatalf("应该找到1条日志: got %d", len(logs))
			}

			if logs[0].Actor != tc.actor {
				t.Errorf("Actor不匹配: got %q, want %q", logs[0].Actor, tc.actor)
			}

			if logs[0].Detail != tc.detail {
				t.Errorf("Detail不匹配: got %q, want %q", logs[0].Detail, tc.detail)
			}
		})
	}
}

// TestAuditLogTimestampPrecision 测试时间戳精度
func TestAuditLogTimestampPrecision(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	now := time.Now().UTC()

	err := ms.WriteAuditLog(&AuditLog{
		Timestamp: now,
		Action:    AuditActionLogin,
		Success:   true,
	})
	if err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	logs, _, err := ms.QueryAuditLogs(&AuditLogQuery{Limit: 1})
	if err != nil {
		t.Fatalf("查询日志失败: %v", err)
	}

	if len(logs) != 1 {
		t.Fatal("应该返回1条日志")
	}

	// SQLite 的 DATETIME 精度到秒，所以我们比较到秒级别
	if !logs[0].Timestamp.Truncate(time.Second).Equal(now.Truncate(time.Second)) {
		t.Errorf("时间戳不匹配: got %v, want %v", logs[0].Timestamp, now)
	}
}

// TestAuditLogEmptyQuery 测试空查询条件
func TestAuditLogEmptyQuery(t *testing.T) {
	ms, cleanup := setupAuditTest(t)
	defer cleanup()

	// 写入一些测试数据
	for i := 0; i < 5; i++ {
		ms.WriteAuditLog(&AuditLog{
			Action:  AuditActionLogin,
			Success: true,
		})
	}

	// 空查询条件应该返回所有数据
	logs, total, err := ms.QueryAuditLogs(&AuditLogQuery{})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	if total != 5 {
		t.Errorf("总数不匹配: got %d, want 5", total)
	}

	// 空查询使用默认 Limit 100
	if len(logs) != 5 {
		t.Errorf("返回数量不匹配: got %d, want 5", len(logs))
	}
}

// boolPtr 返回bool指针
func boolPtr(b bool) *bool {
	return &b
}

// timePtr 返回time指针
func timePtr(t time.Time) *time.Time {
	return &t
}

// BenchmarkWriteAuditLog 审计日志写入性能基准测试
func BenchmarkWriteAuditLog(b *testing.B) {
	ms, cleanup := setupAuditTest(&testing.T{})
	defer cleanup()

	log := &AuditLog{
		Action:    AuditActionLogin,
		Actor:     "benchmark_user",
		IP:        "192.168.1.1",
		Resource:  "test",
		Detail:    "{\"benchmark\":true}",
		Success:   true,
		UserAgent: "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.WriteAuditLog(log)
	}
}

// BenchmarkQueryAuditLogs 审计日志查询性能基准测试
func BenchmarkQueryAuditLogs(b *testing.B) {
	ms, cleanup := setupAuditTest(&testing.T{})
	defer cleanup()

	// 准备测试数据
	for i := 0; i < 1000; i++ {
		ms.WriteAuditLog(&AuditLog{
			Action:  AuditActionLogin,
			Actor:   "benchmark_user",
			Success: true,
		})
	}

	query := &AuditLogQuery{
		Actor: "benchmark_user",
		Limit: 100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.QueryAuditLogs(query)
	}
}
