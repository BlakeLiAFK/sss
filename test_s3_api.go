//go:build ignore

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("S3 API 回归测试")
	fmt.Println("========================================")

	// 配置 S3 客户端
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("admin", "admin123", "")),
	)
	if err != nil {
		log.Fatalf("无法加载配置: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://localhost:8080")
		o.UsePathStyle = true
	})

	ctx := context.TODO()
	testBucket := "test-regression-" + fmt.Sprintf("%d", time.Now().Unix())
	testKey := "test-file.txt"
	testContent := "Hello from S3 API regression test!"
	passed := 0
	failed := 0

	// 测试函数
	test := func(name string, fn func() error) {
		fmt.Printf("\n=== %s ===\n", name)
		if err := fn(); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			failed++
		} else {
			fmt.Println("PASSED")
			passed++
		}
	}

	// 1. ListBuckets
	test("1. ListBuckets", func() error {
		result, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err != nil {
			return err
		}
		fmt.Printf("   存储桶数量: %d\n", len(result.Buckets))
		return nil
	})

	// 2. CreateBucket
	test("2. CreateBucket", func() error {
		_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(testBucket),
		})
		return err
	})

	// 3. HeadBucket
	test("3. HeadBucket", func() error {
		_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String(testBucket),
		})
		return err
	})

	// 4. PutObject
	test("4. PutObject", func() error {
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(testBucket),
			Key:         aws.String(testKey),
			Body:        strings.NewReader(testContent),
			ContentType: aws.String("text/plain"),
		})
		return err
	})

	// 5. HeadObject
	test("5. HeadObject", func() error {
		result, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String(testKey),
		})
		if err != nil {
			return err
		}
		fmt.Printf("   大小: %d, 类型: %s\n", aws.ToInt64(result.ContentLength), aws.ToString(result.ContentType))
		return nil
	})

	// 6. GetObject
	test("6. GetObject", func() error {
		result, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String(testKey),
		})
		if err != nil {
			return err
		}
		defer result.Body.Close()
		data, _ := io.ReadAll(result.Body)
		if string(data) != testContent {
			return fmt.Errorf("内容不匹配: got %q, want %q", string(data), testContent)
		}
		fmt.Printf("   内容验证: OK\n")
		return nil
	})

	// 7. ListObjects
	test("7. ListObjects", func() error {
		result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(testBucket),
		})
		if err != nil {
			return err
		}
		fmt.Printf("   对象数量: %d\n", len(result.Contents))
		return nil
	})

	// 8. CopyObject
	test("8. CopyObject", func() error {
		_, err := client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String("test-copy.txt"),
			CopySource: aws.String(testBucket + "/" + testKey),
		})
		return err
	})

	// 9. DeleteObject (删除复制的文件)
	test("9. DeleteObject", func() error {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("test-copy.txt"),
		})
		return err
	})

	// 10. Multipart Upload - 初始化
	var uploadID string
	test("10. InitiateMultipartUpload", func() error {
		result, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
			Bucket:      aws.String(testBucket),
			Key:         aws.String("multipart-test.txt"),
			ContentType: aws.String("text/plain"),
		})
		if err != nil {
			return err
		}
		uploadID = aws.ToString(result.UploadId)
		fmt.Printf("   UploadId: %s\n", uploadID)
		return nil
	})

	// 11. UploadPart
	var etag string
	test("11. UploadPart", func() error {
		partData := bytes.Repeat([]byte("A"), 5*1024*1024) // 5MB
		result, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String("multipart-test.txt"),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(1),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			return err
		}
		etag = aws.ToString(result.ETag)
		fmt.Printf("   ETag: %s\n", etag)
		return nil
	})

	// 12. ListParts
	test("12. ListParts", func() error {
		result, err := client.ListParts(ctx, &s3.ListPartsInput{
			Bucket:   aws.String(testBucket),
			Key:      aws.String("multipart-test.txt"),
			UploadId: aws.String(uploadID),
		})
		if err != nil {
			return err
		}
		fmt.Printf("   分片数量: %d\n", len(result.Parts))
		return nil
	})

	// 13. AbortMultipartUpload
	test("13. AbortMultipartUpload", func() error {
		_, err := client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(testBucket),
			Key:      aws.String("multipart-test.txt"),
			UploadId: aws.String(uploadID),
		})
		return err
	})

	// 14. 删除测试文件
	test("14. DeleteObject (清理)", func() error {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String(testKey),
		})
		return err
	})

	// 15. DeleteBucket
	test("15. DeleteBucket", func() error {
		_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(testBucket),
		})
		return err
	})

	fmt.Println("\n========================================")
	fmt.Printf("测试结果: %d 通过, %d 失败\n", passed, failed)
	fmt.Println("========================================")
}
