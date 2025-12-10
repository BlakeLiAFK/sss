package api

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	appconfig "sss/internal/config"
	"sss/internal/auth"
	"sss/internal/storage"
	"sss/internal/utils"
)

// setupAWSSDKTest 设置AWS SDK兼容性测试环境
func setupAWSSDKTest(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	// 初始化日志系统
	utils.InitLogger("info")

	tmpDir, err := os.MkdirTemp("", "sss-awssdk-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	dbPath := tmpDir + "/metadata.db"
	dataPath := tmpDir + "/data"

	// 创建存储
	metadata, err := storage.NewMetadataStore(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("创建元数据存储失败: %v", err)
	}

	filestore, err := storage.NewFileStore(dataPath)
	if err != nil {
		metadata.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 初始化全局配置
	appconfig.Global = &appconfig.Config{
		Auth: appconfig.AuthConfig{
			AccessKeyID:     testAccessKey,
			SecretAccessKey: testSecretKey,
		},
		Server: appconfig.ServerConfig{
			Host:   "localhost",
			Port:   8080,
			Region: testRegion,
		},
	}

	// 初始化API Key缓存
	auth.InitAPIKeyCache(metadata)

	// 创建服务器
	server := NewServer(metadata, filestore)

	// 创建测试HTTP服务器
	ts := httptest.NewServer(server)

	cleanup := func() {
		ts.Close()
		metadata.Close()
		os.RemoveAll(tmpDir)
	}

	return ts, cleanup
}

// createS3Client 创建S3客户端
func createS3Client(endpoint string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(testRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			testAccessKey,
			testSecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // 使用路径风格URL
		o.BaseEndpoint = aws.String(endpoint)
	}), nil
}

// TestAWSSDKPutObject 使用AWS SDK测试PutObject
func TestAWSSDKPutObject(t *testing.T) {
	ts, cleanup := setupAWSSDKTest(t)
	defer cleanup()

	client, err := createS3Client(ts.URL)
	if err != nil {
		t.Fatalf("创建S3客户端失败: %v", err)
	}

	ctx := context.Background()

	// 1. 创建bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("sdk-test-bucket"),
	})
	if err != nil {
		t.Fatalf("CreateBucket失败: %v", err)
	}

	// 2. 上传对象
	objectContent := []byte("Hello from AWS SDK!")
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String("sdk-test-bucket"),
		Key:         aws.String("sdk-test-object.txt"),
		Body:        bytes.NewReader(objectContent),
		ContentType: aws.String("text/plain"),
	})

	if err != nil {
		t.Errorf("AWS SDK PutObject失败: %v", err)
		t.Logf("这可能表明签名兼容性问题")
	} else {
		t.Log("AWS SDK PutObject成功!")
	}

	// 3. 获取对象验证
	getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("sdk-test-bucket"),
		Key:    aws.String("sdk-test-object.txt"),
	})
	if err != nil {
		t.Errorf("AWS SDK GetObject失败: %v", err)
	} else {
		defer getOutput.Body.Close()
		body, _ := io.ReadAll(getOutput.Body)
		if !bytes.Equal(body, objectContent) {
			t.Errorf("内容不匹配: 期望 %s, 实际 %s", objectContent, body)
		} else {
			t.Log("AWS SDK GetObject成功!")
		}
	}
}

// TestAWSSDKListBuckets 使用AWS SDK测试ListBuckets
func TestAWSSDKListBuckets(t *testing.T) {
	ts, cleanup := setupAWSSDKTest(t)
	defer cleanup()

	client, err := createS3Client(ts.URL)
	if err != nil {
		t.Fatalf("创建S3客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建几个bucket
	buckets := []string{"list-bucket-1", "list-bucket-2", "list-bucket-3"}
	for _, bucket := range buckets {
		_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {
			t.Fatalf("CreateBucket %s 失败: %v", bucket, err)
		}
	}

	// ListBuckets
	output, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		t.Errorf("AWS SDK ListBuckets失败: %v", err)
	} else {
		t.Logf("ListBuckets返回 %d 个bucket", len(output.Buckets))
		for _, b := range output.Buckets {
			t.Logf("  - %s", *b.Name)
		}
	}
}

// TestAWSSDKListObjects 使用AWS SDK测试ListObjects
func TestAWSSDKListObjects(t *testing.T) {
	ts, cleanup := setupAWSSDKTest(t)
	defer cleanup()

	client, err := createS3Client(ts.URL)
	if err != nil {
		t.Fatalf("创建S3客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("list-objects-bucket"),
	})
	if err != nil {
		t.Fatalf("CreateBucket失败: %v", err)
	}

	// 上传多个对象
	objects := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
	for _, obj := range objects {
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String("list-objects-bucket"),
			Key:    aws.String(obj),
			Body:   strings.NewReader("Content of " + obj),
		})
		if err != nil {
			t.Fatalf("PutObject %s 失败: %v", obj, err)
		}
	}

	// ListObjects
	output, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String("list-objects-bucket"),
	})
	if err != nil {
		t.Errorf("AWS SDK ListObjectsV2失败: %v", err)
	} else {
		t.Logf("ListObjects返回 %d 个对象", len(output.Contents))
		for _, obj := range output.Contents {
			t.Logf("  - %s (size: %d)", *obj.Key, *obj.Size)
		}
	}
}

// TestAWSSDKDeleteObject 使用AWS SDK测试DeleteObject
func TestAWSSDKDeleteObject(t *testing.T) {
	ts, cleanup := setupAWSSDKTest(t)
	defer cleanup()

	client, err := createS3Client(ts.URL)
	if err != nil {
		t.Fatalf("创建S3客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("delete-test-bucket"),
	})
	if err != nil {
		t.Fatalf("CreateBucket失败: %v", err)
	}

	// 上传对象
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("delete-test-bucket"),
		Key:    aws.String("to-delete.txt"),
		Body:   strings.NewReader("This will be deleted"),
	})
	if err != nil {
		t.Fatalf("PutObject失败: %v", err)
	}

	// 删除对象
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String("delete-test-bucket"),
		Key:    aws.String("to-delete.txt"),
	})
	if err != nil {
		t.Errorf("AWS SDK DeleteObject失败: %v", err)
	} else {
		t.Log("AWS SDK DeleteObject成功!")
	}

	// 验证对象已删除
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String("delete-test-bucket"),
		Key:    aws.String("to-delete.txt"),
	})
	if err == nil {
		t.Error("对象应该已被删除")
	}
}

// TestAWSSDKCopyObject 使用AWS SDK测试CopyObject
func TestAWSSDKCopyObject(t *testing.T) {
	ts, cleanup := setupAWSSDKTest(t)
	defer cleanup()

	client, err := createS3Client(ts.URL)
	if err != nil {
		t.Fatalf("创建S3客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("copy-test-bucket"),
	})
	if err != nil {
		t.Fatalf("CreateBucket失败: %v", err)
	}

	// 上传源对象
	sourceContent := "This is the source content"
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("copy-test-bucket"),
		Key:    aws.String("source.txt"),
		Body:   strings.NewReader(sourceContent),
	})
	if err != nil {
		t.Fatalf("PutObject失败: %v", err)
	}

	// 复制对象
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String("copy-test-bucket"),
		Key:        aws.String("destination.txt"),
		CopySource: aws.String("copy-test-bucket/source.txt"),
	})
	if err != nil {
		t.Errorf("AWS SDK CopyObject失败: %v", err)
	} else {
		t.Log("AWS SDK CopyObject成功!")
	}

	// 验证复制的对象
	getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("copy-test-bucket"),
		Key:    aws.String("destination.txt"),
	})
	if err != nil {
		t.Errorf("获取复制对象失败: %v", err)
	} else {
		defer getOutput.Body.Close()
		body, _ := io.ReadAll(getOutput.Body)
		if string(body) != sourceContent {
			t.Errorf("复制内容不匹配")
		}
	}
}

// TestAWSSDKHeadObject 使用AWS SDK测试HeadObject
func TestAWSSDKHeadObject(t *testing.T) {
	ts, cleanup := setupAWSSDKTest(t)
	defer cleanup()

	client, err := createS3Client(ts.URL)
	if err != nil {
		t.Fatalf("创建S3客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("head-test-bucket"),
	})
	if err != nil {
		t.Fatalf("CreateBucket失败: %v", err)
	}

	// 上传对象
	content := "Content for HEAD test"
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String("head-test-bucket"),
		Key:         aws.String("head-test.txt"),
		Body:        strings.NewReader(content),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		t.Fatalf("PutObject失败: %v", err)
	}

	// HeadObject
	output, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String("head-test-bucket"),
		Key:    aws.String("head-test.txt"),
	})
	if err != nil {
		t.Errorf("AWS SDK HeadObject失败: %v", err)
	} else {
		t.Logf("HeadObject成功: ContentLength=%d, ContentType=%s, ETag=%s",
			*output.ContentLength, *output.ContentType, *output.ETag)
	}
}

// TestAWSSDKMultipartUpload 使用AWS SDK测试多段上传
func TestAWSSDKMultipartUpload(t *testing.T) {
	ts, cleanup := setupAWSSDKTest(t)
	defer cleanup()

	client, err := createS3Client(ts.URL)
	if err != nil {
		t.Fatalf("创建S3客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("multipart-bucket"),
	})
	if err != nil {
		t.Fatalf("CreateBucket失败: %v", err)
	}

	// 初始化多段上传
	initOutput, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String("multipart-bucket"),
		Key:    aws.String("multipart-object.bin"),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload失败: %v", err)
	}
	t.Logf("InitiateMultipartUpload成功: UploadId=%s", *initOutput.UploadId)

	// 上传分片
	partSize := 5 * 1024 * 1024 // 5MB - S3最小分片大小
	content := make([]byte, partSize)
	for i := range content {
		content[i] = byte(i % 256)
	}

	uploadPartOutput, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String("multipart-bucket"),
		Key:        aws.String("multipart-object.bin"),
		UploadId:   initOutput.UploadId,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(content),
	})
	if err != nil {
		t.Errorf("UploadPart失败: %v", err)
		// 中止上传
		client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String("multipart-bucket"),
			Key:      aws.String("multipart-object.bin"),
			UploadId: initOutput.UploadId,
		})
		return
	}
	t.Logf("UploadPart成功: ETag=%s", *uploadPartOutput.ETag)

	// 完成多段上传
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String("multipart-bucket"),
		Key:      aws.String("multipart-object.bin"),
		UploadId: initOutput.UploadId,
		MultipartUpload: &s3Types.CompletedMultipartUpload{
			Parts: []s3Types.CompletedPart{
				{
					PartNumber: aws.Int32(1),
					ETag:       uploadPartOutput.ETag,
				},
			},
		},
	})
	if err != nil {
		t.Errorf("CompleteMultipartUpload失败: %v", err)
	} else {
		t.Log("CompleteMultipartUpload成功!")
	}
}
