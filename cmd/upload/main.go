package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"source-access-key",
			"source-secret-key",
			"",
		)),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://localhost:9001")
		o.UsePathStyle = true
	})

	// 上传测试文件
	files := map[string]string{
		"test1.txt":        "Test file 1 content",
		"folder/test2.txt": "Test file 2 content",
		"folder/data.json": `{"name": "test", "value": 123}`,
	}

	for key, content := range files {
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String("source-bucket"),
			Key:    aws.String(key),
			Body:   bytes.NewReader([]byte(content)),
		})
		if err != nil {
			log.Printf("Failed to upload %s: %v", key, err)
		} else {
			fmt.Printf("Uploaded: %s\n", key)
		}
	}

	// 列出对象
	fmt.Println("\n=== Source bucket contents ===")
	result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String("source-bucket"),
	})
	if err != nil {
		log.Fatalf("Failed to list objects: %v", err)
	}

	for _, obj := range result.Contents {
		fmt.Printf("  %s (%d bytes)\n", *obj.Key, *obj.Size)
	}
}
