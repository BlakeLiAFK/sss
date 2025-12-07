package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// MigrateConfig 迁移配置
type MigrateConfig struct {
	SourceEndpoint  string `json:"sourceEndpoint"`
	SourceAccessKey string `json:"sourceAccessKey"`
	SourceSecretKey string `json:"sourceSecretKey"`
	SourceBucket    string `json:"sourceBucket"`
	SourcePrefix    string `json:"sourcePrefix"`    // 可选：只迁移指定前缀的对象
	SourceRegion    string `json:"sourceRegion"`    // 可选：源服务区域
	TargetBucket    string `json:"targetBucket"`
	TargetPrefix    string `json:"targetPrefix"`    // 可选：目标前缀
	OverwriteExist  bool   `json:"overwriteExist"`  // 是否覆盖已存在的文件
}

// MigrateProgress 迁移进度
type MigrateProgress struct {
	JobID         string     `json:"jobId"`
	Status        string     `json:"status"` // pending, running, completed, failed, cancelled
	TotalObjects  int        `json:"totalObjects"`
	Completed     int        `json:"completed"`
	Failed        int        `json:"failed"`
	Skipped       int        `json:"skipped"`     // 跳过的已存在文件
	TotalSize     int64      `json:"totalSize"`   // 总字节数
	TransferSize  int64      `json:"transferSize"` // 已传输字节数
	CurrentFile   string     `json:"currentFile,omitempty"`
	StartTime     time.Time  `json:"startTime"`
	EndTime       *time.Time `json:"endTime,omitempty"`
	Error         string     `json:"error,omitempty"`
	FailedObjects []string   `json:"failedObjects,omitempty"` // 失败的对象列表
	Config        MigrateConfig `json:"config"`
}

// MigrateManager 迁移任务管理器
type MigrateManager struct {
	mu       sync.RWMutex
	jobs     map[string]*MigrateProgress
	metadata *MetadataStore
	fileStore *FileStore
}

// 全局迁移管理器
var migrateManager *MigrateManager
var migrateOnce sync.Once

// GetMigrateManager 获取迁移管理器单例
func GetMigrateManager(metadata *MetadataStore, fileStore *FileStore) *MigrateManager {
	migrateOnce.Do(func() {
		migrateManager = &MigrateManager{
			jobs:     make(map[string]*MigrateProgress),
			metadata: metadata,
			fileStore: fileStore,
		}
	})
	return migrateManager
}

// ResetMigrateManagerForTest 重置迁移管理器（仅用于测试）
// 注意：此函数不是线程安全的，仅应在测试初始化时调用
func ResetMigrateManagerForTest() {
	migrateOnce = sync.Once{}
	migrateManager = nil
}

// generateJobID 生成任务ID
func generateJobID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// StartMigration 启动迁移任务
func (m *MigrateManager) StartMigration(cfg MigrateConfig) (string, error) {
	// 验证配置
	if cfg.SourceEndpoint == "" {
		return "", fmt.Errorf("sourceEndpoint is required")
	}
	if cfg.SourceAccessKey == "" || cfg.SourceSecretKey == "" {
		return "", fmt.Errorf("source credentials are required")
	}
	if cfg.SourceBucket == "" {
		return "", fmt.Errorf("sourceBucket is required")
	}
	if cfg.TargetBucket == "" {
		return "", fmt.Errorf("targetBucket is required")
	}

	// 检查目标桶是否存在
	bucket, err := m.metadata.GetBucket(cfg.TargetBucket)
	if err != nil {
		return "", fmt.Errorf("failed to check target bucket: %w", err)
	}
	if bucket == nil {
		return "", fmt.Errorf("target bucket not found: %s", cfg.TargetBucket)
	}

	// 设置默认区域
	if cfg.SourceRegion == "" {
		cfg.SourceRegion = "us-east-1"
	}

	// 生成任务ID
	jobID := generateJobID()

	// 创建进度记录
	progress := &MigrateProgress{
		JobID:     jobID,
		Status:    "pending",
		StartTime: time.Now(),
		Config:    cfg,
	}

	m.mu.Lock()
	m.jobs[jobID] = progress
	m.mu.Unlock()

	// 启动后台任务
	go m.runMigration(jobID, cfg)

	return jobID, nil
}

// GetProgress 获取迁移进度
func (m *MigrateManager) GetProgress(jobID string) *MigrateProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[jobID]
}

// GetAllJobs 获取所有任务
func (m *MigrateManager) GetAllJobs() []*MigrateProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MigrateProgress, 0, len(m.jobs))
	for _, job := range m.jobs {
		result = append(result, job)
	}
	return result
}

// CancelMigration 取消迁移任务
func (m *MigrateManager) CancelMigration(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
		return fmt.Errorf("job already finished")
	}

	job.Status = "cancelled"
	now := time.Now()
	job.EndTime = &now
	return nil
}

// DeleteJob 删除任务记录
func (m *MigrateManager) DeleteJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status == "running" || job.Status == "pending" {
		return fmt.Errorf("cannot delete running job")
	}

	delete(m.jobs, jobID)
	return nil
}

// runMigration 执行迁移
func (m *MigrateManager) runMigration(jobID string, cfg MigrateConfig) {
	m.mu.Lock()
	progress := m.jobs[jobID]
	progress.Status = "running"
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		if progress.Status == "running" {
			if progress.Failed > 0 {
				progress.Status = "completed"
				progress.Error = fmt.Sprintf("%d objects failed", progress.Failed)
			} else {
				progress.Status = "completed"
			}
		}
		now := time.Now()
		progress.EndTime = &now
		m.mu.Unlock()
	}()

	// 创建 S3 客户端
	ctx := context.Background()
	s3Client, err := m.createS3Client(ctx, cfg)
	if err != nil {
		m.setError(progress, fmt.Sprintf("failed to create S3 client: %v", err))
		return
	}

	// 列出源桶对象
	objects, err := m.listSourceObjects(ctx, s3Client, cfg)
	if err != nil {
		m.setError(progress, fmt.Sprintf("failed to list source objects: %v", err))
		return
	}

	m.mu.Lock()
	progress.TotalObjects = len(objects)
	m.mu.Unlock()

	if len(objects) == 0 {
		slog.Info("迁移任务完成，无对象需要迁移", "jobId", jobID)
		return
	}

	// 逐个迁移对象
	for _, obj := range objects {
		// 检查是否被取消
		m.mu.RLock()
		if progress.Status == "cancelled" {
			m.mu.RUnlock()
			return
		}
		m.mu.RUnlock()

		// 更新当前文件
		m.mu.Lock()
		progress.CurrentFile = obj.Key
		m.mu.Unlock()

		// 计算目标 key
		targetKey := obj.Key
		if cfg.SourcePrefix != "" && cfg.TargetPrefix != "" {
			// 替换前缀
			targetKey = cfg.TargetPrefix + obj.Key[len(cfg.SourcePrefix):]
		} else if cfg.TargetPrefix != "" {
			targetKey = cfg.TargetPrefix + obj.Key
		}

		// 检查目标是否已存在
		if !cfg.OverwriteExist {
			existingObj, _ := m.metadata.GetObject(cfg.TargetBucket, targetKey)
			if existingObj != nil {
				m.mu.Lock()
				progress.Skipped++
				progress.Completed++
				m.mu.Unlock()
				continue
			}
		}

		// 下载并上传对象
		err := m.transferObject(ctx, s3Client, cfg, obj.Key, targetKey, obj.Size)
		if err != nil {
			slog.Error("迁移对象失败",
				"jobId", jobID,
				"key", obj.Key,
				"error", err)
			m.mu.Lock()
			progress.Failed++
			progress.FailedObjects = append(progress.FailedObjects, obj.Key)
			m.mu.Unlock()
		} else {
			m.mu.Lock()
			progress.Completed++
			progress.TransferSize += obj.Size
			m.mu.Unlock()
		}
	}

	slog.Info("迁移任务完成",
		"jobId", jobID,
		"total", progress.TotalObjects,
		"completed", progress.Completed,
		"failed", progress.Failed,
		"skipped", progress.Skipped)
}

// sourceObject 源对象信息
type sourceObject struct {
	Key  string
	Size int64
	ETag string
}

// createS3Client 创建源 S3 客户端
func (m *MigrateManager) createS3Client(ctx context.Context, cfg MigrateConfig) (*s3.Client, error) {
	// 创建自定义凭证提供程序
	creds := credentials.NewStaticCredentialsProvider(
		cfg.SourceAccessKey,
		cfg.SourceSecretKey,
		"",
	)

	// 创建自定义端点解析器
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.SourceEndpoint,
				HostnameImmutable: true,
			}, nil
		},
	)

	// 加载配置
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.SourceRegion),
		config.WithCredentialsProvider(creds),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, err
	}

	// 创建 S3 客户端，使用 path-style（兼容大多数 S3 兼容服务）
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return client, nil
}

// listSourceObjects 列出源桶中的所有对象
func (m *MigrateManager) listSourceObjects(ctx context.Context, client *s3.Client, cfg MigrateConfig) ([]sourceObject, error) {
	var objects []sourceObject
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(cfg.SourceBucket),
		}
		if cfg.SourcePrefix != "" {
			input.Prefix = aws.String(cfg.SourcePrefix)
		}
		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		resp, err := client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, obj := range resp.Contents {
			objects = append(objects, sourceObject{
				Key:  aws.ToString(obj.Key),
				Size: aws.ToInt64(obj.Size),
				ETag: aws.ToString(obj.ETag),
			})
		}

		if !aws.ToBool(resp.IsTruncated) {
			break
		}
		continuationToken = resp.NextContinuationToken
	}

	return objects, nil
}

// transferObject 传输单个对象
func (m *MigrateManager) transferObject(ctx context.Context, client *s3.Client, cfg MigrateConfig, sourceKey, targetKey string, size int64) error {
	// 从源下载
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.SourceBucket),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer getResp.Body.Close()

	// 获取 Content-Type
	contentType := "application/octet-stream"
	if getResp.ContentType != nil {
		contentType = *getResp.ContentType
	}

	// 存储到本地
	storagePath, etag, err := m.fileStore.PutObject(cfg.TargetBucket, targetKey, getResp.Body, size)
	if err != nil {
		return fmt.Errorf("failed to store object: %w", err)
	}

	// 保存元数据
	obj := &Object{
		Bucket:       cfg.TargetBucket,
		Key:          targetKey,
		Size:         size,
		ETag:         etag,
		ContentType:  contentType,
		StoragePath:  storagePath,
		LastModified: time.Now(),
	}
	err = m.metadata.PutObject(obj)
	if err != nil {
		// 清理已存储的文件
		m.fileStore.DeleteObject(storagePath)
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// setError 设置错误状态
func (m *MigrateManager) setError(progress *MigrateProgress, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	progress.Status = "failed"
	progress.Error = errMsg
	now := time.Now()
	progress.EndTime = &now
}

// ValidateMigrateConfig 验证迁移配置（连接测试）
func (m *MigrateManager) ValidateMigrateConfig(cfg MigrateConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 设置默认区域
	if cfg.SourceRegion == "" {
		cfg.SourceRegion = "us-east-1"
	}

	// 创建客户端
	client, err := m.createS3Client(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// 尝试列出对象（只取1个）
	_, err = client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(cfg.SourceBucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("failed to access bucket: %w", err)
	}

	return nil
}

// GetJobStats 获取任务统计
func (m *MigrateManager) GetJobStats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]int{
		"total":     len(m.jobs),
		"pending":   0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"cancelled": 0,
	}

	for _, job := range m.jobs {
		stats[job.Status]++
	}

	return stats
}
