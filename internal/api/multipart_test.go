package api

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// setupMultipartTestServer 初始化多部分上传测试服务器
func setupMultipartTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := t.TempDir()
	metadata, err := storage.NewMetadataStore(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("创建MetadataStore失败: %v", err)
	}

	filestore, err := storage.NewFileStore(tempDir)
	if err != nil {
		metadata.Close()
		t.Fatalf("创建FileStore失败: %v", err)
	}

	server := NewServer(metadata, filestore)
	cleanup := func() {
		metadata.Close()
	}

	return server, cleanup
}

// TestHandleInitiateMultipartUpload 测试初始化多部分上传
func TestHandleInitiateMultipartUpload(t *testing.T) {
	server, cleanup := setupMultipartTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("multipart-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	tests := []struct {
		name           string
		bucket         string
		key            string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "初始化普通上传",
			bucket:         "multipart-bucket",
			key:            "large-file.bin",
			contentType:    "application/octet-stream",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "初始化带自定义ContentType",
			bucket:         "multipart-bucket",
			key:            "video.mp4",
			contentType:    "video/mp4",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "不存在的桶",
			bucket:         "nonexistent-bucket",
			key:            "file.bin",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "中文路径对象",
			bucket:         "multipart-bucket",
			key:            "文档/大文件.zip",
			contentType:    "application/zip",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/"+tc.bucket+"/"+tc.key+"?uploads", nil)
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			server.handleInitiateMultipartUpload(rec, req, tc.bucket, tc.key)

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d, 响应: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			if tc.expectedStatus == http.StatusOK {
				// 验证响应格式
				var result InitiateMultipartUploadResult
				if err := xml.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Errorf("解析响应失败: %v", err)
				}
				if result.Bucket != tc.bucket {
					t.Errorf("Bucket错误: 期望 %s, 实际 %s", tc.bucket, result.Bucket)
				}
				if result.Key != tc.key {
					t.Errorf("Key错误: 期望 %s, 实际 %s", tc.key, result.Key)
				}
				if result.UploadId == "" {
					t.Error("UploadId不应为空")
				}
			}
		})
	}
}

// TestHandleUploadPart 测试上传分片
func TestHandleUploadPart(t *testing.T) {
	server, cleanup := setupMultipartTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("part-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 通过API初始化多部分上传以获取有效的uploadID
	initReq := httptest.NewRequest(http.MethodPost, "/part-bucket/test-file.bin?uploads", nil)
	initRec := httptest.NewRecorder()
	server.handleInitiateMultipartUpload(initRec, initReq, "part-bucket", "test-file.bin")

	if initRec.Code != http.StatusOK {
		t.Fatalf("初始化上传失败: %d", initRec.Code)
	}

	var initResult InitiateMultipartUploadResult
	xml.Unmarshal(initRec.Body.Bytes(), &initResult)
	validUploadID := initResult.UploadId

	tests := []struct {
		name           string
		uploadID       string
		partNumber     string
		content        []byte
		expectedStatus int
	}{
		{
			name:           "上传第一个分片",
			uploadID:       validUploadID,
			partNumber:     "1",
			content:        []byte("Part 1 content"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "上传第二个分片",
			uploadID:       validUploadID,
			partNumber:     "2",
			content:        []byte("Part 2 content with more data"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "分片号边界-最小值1",
			uploadID:       validUploadID,
			partNumber:     "1",
			content:        []byte("Min part"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "分片号边界-最大值10000",
			uploadID:       validUploadID,
			partNumber:     "10000",
			content:        []byte("Max part"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "无效分片号-0",
			uploadID:       validUploadID,
			partNumber:     "0",
			content:        []byte("Invalid"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "无效分片号-超过10000",
			uploadID:       validUploadID,
			partNumber:     "10001",
			content:        []byte("Invalid"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "无效分片号-非数字",
			uploadID:       validUploadID,
			partNumber:     "abc",
			content:        []byte("Invalid"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "不存在的上传ID",
			uploadID:       "nonexistent-upload-id",
			partNumber:     "1",
			content:        []byte("Content"),
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/part-bucket/test-file.bin?uploadId=" + tc.uploadID + "&partNumber=" + tc.partNumber
			req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(tc.content))
			rec := httptest.NewRecorder()

			server.handleUploadPart(rec, req, "part-bucket", "test-file.bin", tc.uploadID)

			if rec.Code != tc.expectedStatus {
				t.Errorf("状态码错误: 期望 %d, 实际 %d, 响应: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			if tc.expectedStatus == http.StatusOK {
				etag := rec.Header().Get("ETag")
				if etag == "" {
					t.Error("成功上传分片后应返回ETag")
				}
			}
		})
	}
}

// TestHandleCompleteMultipartUpload 测试完成多部分上传
func TestHandleCompleteMultipartUpload(t *testing.T) {
	server, cleanup := setupMultipartTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("complete-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	t.Run("成功完成多部分上传", func(t *testing.T) {
		// 通过API初始化上传
		initReq := httptest.NewRequest(http.MethodPost, "/complete-bucket/completed-file.bin?uploads", nil)
		initRec := httptest.NewRecorder()
		server.handleInitiateMultipartUpload(initRec, initReq, "complete-bucket", "completed-file.bin")

		if initRec.Code != http.StatusOK {
			t.Fatalf("初始化上传失败: %d", initRec.Code)
		}

		var initResult InitiateMultipartUploadResult
		xml.Unmarshal(initRec.Body.Bytes(), &initResult)
		uploadID := initResult.UploadId

		// 通过API上传分片
		part1Content := bytes.Repeat([]byte("A"), 1024)
		part2Content := bytes.Repeat([]byte("B"), 1024)

		part1Req := httptest.NewRequest(http.MethodPut, "/complete-bucket/completed-file.bin?uploadId="+uploadID+"&partNumber=1", bytes.NewReader(part1Content))
		part1Rec := httptest.NewRecorder()
		server.handleUploadPart(part1Rec, part1Req, "complete-bucket", "completed-file.bin", uploadID)
		if part1Rec.Code != http.StatusOK {
			t.Fatalf("上传分片1失败: %d", part1Rec.Code)
		}
		etag1 := strings.Trim(part1Rec.Header().Get("ETag"), `"`)

		part2Req := httptest.NewRequest(http.MethodPut, "/complete-bucket/completed-file.bin?uploadId="+uploadID+"&partNumber=2", bytes.NewReader(part2Content))
		part2Rec := httptest.NewRecorder()
		server.handleUploadPart(part2Rec, part2Req, "complete-bucket", "completed-file.bin", uploadID)
		if part2Rec.Code != http.StatusOK {
			t.Fatalf("上传分片2失败: %d", part2Rec.Code)
		}
		etag2 := strings.Trim(part2Rec.Header().Get("ETag"), `"`)

		// 发送完成请求
		completeReq := `<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUpload>
  <Part><PartNumber>1</PartNumber><ETag>"` + etag1 + `"</ETag></Part>
  <Part><PartNumber>2</PartNumber><ETag>"` + etag2 + `"</ETag></Part>
</CompleteMultipartUpload>`

		req := httptest.NewRequest(http.MethodPost, "/complete-bucket/completed-file.bin?uploadId="+uploadID, strings.NewReader(completeReq))
		rec := httptest.NewRecorder()

		server.handleCompleteMultipartUpload(rec, req, "complete-bucket", "completed-file.bin", uploadID)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, 响应: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		// 验证响应
		var result CompleteMultipartUploadResult
		if err := xml.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Errorf("解析响应失败: %v", err)
		}
		if result.Bucket != "complete-bucket" {
			t.Errorf("Bucket错误: %s", result.Bucket)
		}
		if result.Key != "completed-file.bin" {
			t.Errorf("Key错误: %s", result.Key)
		}
		if result.ETag == "" {
			t.Error("ETag不应为空")
		}

		// 验证对象已创建
		obj, err := server.metadata.GetObject("complete-bucket", "completed-file.bin")
		if err != nil {
			t.Errorf("获取对象失败: %v", err)
		}
		if obj == nil {
			t.Error("对象应已创建")
		}
	})

	t.Run("不存在的上传ID", func(t *testing.T) {
		completeReq := `<CompleteMultipartUpload></CompleteMultipartUpload>`
		req := httptest.NewRequest(http.MethodPost, "/complete-bucket/file.bin?uploadId=nonexistent", strings.NewReader(completeReq))
		rec := httptest.NewRecorder()

		server.handleCompleteMultipartUpload(rec, req, "complete-bucket", "file.bin", "nonexistent")

		if rec.Code != http.StatusNotFound {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("无效的分片号", func(t *testing.T) {
		// 初始化上传但不上传分片
		upload := &storage.MultipartUpload{
			UploadID:    "invalid-part-upload",
			Bucket:      "complete-bucket",
			Key:         "invalid-parts.bin",
			ContentType: "application/octet-stream",
		}
		server.metadata.CreateMultipartUpload(upload)

		completeReq := `<CompleteMultipartUpload>
  <Part><PartNumber>99</PartNumber><ETag>"fake-etag"</ETag></Part>
</CompleteMultipartUpload>`

		req := httptest.NewRequest(http.MethodPost, "/complete-bucket/invalid-parts.bin?uploadId=invalid-part-upload", strings.NewReader(completeReq))
		rec := httptest.NewRecorder()

		server.handleCompleteMultipartUpload(rec, req, "complete-bucket", "invalid-parts.bin", "invalid-part-upload")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("ETag不匹配", func(t *testing.T) {
		// 通过API初始化上传
		initReq := httptest.NewRequest(http.MethodPost, "/complete-bucket/etag-mismatch.bin?uploads", nil)
		initRec := httptest.NewRecorder()
		server.handleInitiateMultipartUpload(initRec, initReq, "complete-bucket", "etag-mismatch.bin")
		var initResult InitiateMultipartUploadResult
		xml.Unmarshal(initRec.Body.Bytes(), &initResult)
		uploadID := initResult.UploadId

		// 通过API上传一个分片
		partReq := httptest.NewRequest(http.MethodPut, "/complete-bucket/etag-mismatch.bin?uploadId="+uploadID+"&partNumber=1", bytes.NewReader([]byte("data")))
		partRec := httptest.NewRecorder()
		server.handleUploadPart(partRec, partReq, "complete-bucket", "etag-mismatch.bin", uploadID)

		// 使用错误的ETag完成
		completeReq := `<CompleteMultipartUpload>
  <Part><PartNumber>1</PartNumber><ETag>"wrong-etag"</ETag></Part>
</CompleteMultipartUpload>`

		req := httptest.NewRequest(http.MethodPost, "/complete-bucket/etag-mismatch.bin?uploadId="+uploadID, strings.NewReader(completeReq))
		rec := httptest.NewRecorder()

		server.handleCompleteMultipartUpload(rec, req, "complete-bucket", "etag-mismatch.bin", uploadID)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("无效的XML请求", func(t *testing.T) {
		// 通过API初始化上传
		initReq := httptest.NewRequest(http.MethodPost, "/complete-bucket/invalid-xml.bin?uploads", nil)
		initRec := httptest.NewRecorder()
		server.handleInitiateMultipartUpload(initRec, initReq, "complete-bucket", "invalid-xml.bin")
		var initResult InitiateMultipartUploadResult
		xml.Unmarshal(initRec.Body.Bytes(), &initResult)
		uploadID := initResult.UploadId

		req := httptest.NewRequest(http.MethodPost, "/complete-bucket/invalid-xml.bin?uploadId="+uploadID, strings.NewReader("not valid xml"))
		rec := httptest.NewRecorder()

		server.handleCompleteMultipartUpload(rec, req, "complete-bucket", "invalid-xml.bin", uploadID)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestHandleAbortMultipartUpload 测试中止多部分上传
func TestHandleAbortMultipartUpload(t *testing.T) {
	server, cleanup := setupMultipartTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("abort-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	t.Run("成功中止上传", func(t *testing.T) {
		// 通过API初始化上传
		initReq := httptest.NewRequest(http.MethodPost, "/abort-bucket/abort-file.bin?uploads", nil)
		initRec := httptest.NewRecorder()
		server.handleInitiateMultipartUpload(initRec, initReq, "abort-bucket", "abort-file.bin")
		var initResult InitiateMultipartUploadResult
		xml.Unmarshal(initRec.Body.Bytes(), &initResult)
		uploadID := initResult.UploadId

		// 通过API上传一些分片
		part1Req := httptest.NewRequest(http.MethodPut, "/abort-bucket/abort-file.bin?uploadId="+uploadID+"&partNumber=1", bytes.NewReader([]byte("part 1")))
		part1Rec := httptest.NewRecorder()
		server.handleUploadPart(part1Rec, part1Req, "abort-bucket", "abort-file.bin", uploadID)

		part2Req := httptest.NewRequest(http.MethodPut, "/abort-bucket/abort-file.bin?uploadId="+uploadID+"&partNumber=2", bytes.NewReader([]byte("part 2")))
		part2Rec := httptest.NewRecorder()
		server.handleUploadPart(part2Rec, part2Req, "abort-bucket", "abort-file.bin", uploadID)

		req := httptest.NewRequest(http.MethodDelete, "/abort-bucket/abort-file.bin?uploadId="+uploadID, nil)
		rec := httptest.NewRecorder()

		server.handleAbortMultipartUpload(rec, req, "abort-bucket", "abort-file.bin", uploadID)

		if rec.Code != http.StatusNoContent {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNoContent, rec.Code)
		}

		// 验证上传已被删除
		existingUpload, _ := server.metadata.GetMultipartUpload(uploadID)
		if existingUpload != nil {
			t.Error("上传应已被删除")
		}
	})

	t.Run("中止不存在的上传", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/abort-bucket/file.bin?uploadId=nonexistent", nil)
		rec := httptest.NewRecorder()

		server.handleAbortMultipartUpload(rec, req, "abort-bucket", "file.bin", "nonexistent")

		if rec.Code != http.StatusNotFound {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// TestHandleListParts 测试列出分片
func TestHandleListParts(t *testing.T) {
	server, cleanup := setupMultipartTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("list-parts-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	t.Run("列出已上传的分片", func(t *testing.T) {
		// 通过API初始化上传
		initReq := httptest.NewRequest(http.MethodPost, "/list-parts-bucket/list-file.bin?uploads", nil)
		initRec := httptest.NewRecorder()
		server.handleInitiateMultipartUpload(initRec, initReq, "list-parts-bucket", "list-file.bin")
		var initResult InitiateMultipartUploadResult
		xml.Unmarshal(initRec.Body.Bytes(), &initResult)
		uploadID := initResult.UploadId

		// 通过API上传分片
		for i := 1; i <= 3; i++ {
			content := bytes.Repeat([]byte{byte(i)}, 1024)
			partReq := httptest.NewRequest(http.MethodPut, "/list-parts-bucket/list-file.bin?uploadId="+uploadID+"&partNumber="+strconv.Itoa(i), bytes.NewReader(content))
			partRec := httptest.NewRecorder()
			server.handleUploadPart(partRec, partReq, "list-parts-bucket", "list-file.bin", uploadID)
		}

		req := httptest.NewRequest(http.MethodGet, "/list-parts-bucket/list-file.bin?uploadId="+uploadID, nil)
		rec := httptest.NewRecorder()

		server.handleListParts(rec, req, "list-parts-bucket", "list-file.bin", uploadID)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var result ListPartsResult
		if err := xml.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Errorf("解析响应失败: %v", err)
		}

		if len(result.Parts) != 3 {
			t.Errorf("分片数量错误: 期望 3, 实际 %d", len(result.Parts))
		}

		if result.Bucket != "list-parts-bucket" {
			t.Errorf("Bucket错误: %s", result.Bucket)
		}
		if result.UploadId != uploadID {
			t.Errorf("UploadId错误: 期望 %s, 实际 %s", uploadID, result.UploadId)
		}
	})

	t.Run("列出空分片", func(t *testing.T) {
		// 通过API初始化上传（但不上传分片）
		initReq := httptest.NewRequest(http.MethodPost, "/list-parts-bucket/empty-file.bin?uploads", nil)
		initRec := httptest.NewRecorder()
		server.handleInitiateMultipartUpload(initRec, initReq, "list-parts-bucket", "empty-file.bin")
		var initResult InitiateMultipartUploadResult
		xml.Unmarshal(initRec.Body.Bytes(), &initResult)
		uploadID := initResult.UploadId

		req := httptest.NewRequest(http.MethodGet, "/list-parts-bucket/empty-file.bin?uploadId="+uploadID, nil)
		rec := httptest.NewRecorder()

		server.handleListParts(rec, req, "list-parts-bucket", "empty-file.bin", uploadID)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var result ListPartsResult
		xml.Unmarshal(rec.Body.Bytes(), &result)

		if len(result.Parts) != 0 {
			t.Errorf("空上传应无分片: 实际 %d", len(result.Parts))
		}
	})

	t.Run("列出不存在的上传", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/list-parts-bucket/file.bin?uploadId=nonexistent", nil)
		rec := httptest.NewRecorder()

		server.handleListParts(rec, req, "list-parts-bucket", "file.bin", "nonexistent")

		if rec.Code != http.StatusNotFound {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// TestMultipartUploadCompleteFlow 测试多部分上传完整流程
func TestMultipartUploadCompleteFlow(t *testing.T) {
	server, cleanup := setupMultipartTestServer(t)
	defer cleanup()

	// 创建测试桶
	if err := server.metadata.CreateBucket("flow-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	// 1. 初始化上传
	initReq := httptest.NewRequest(http.MethodPost, "/flow-bucket/large-file.bin?uploads", nil)
	initReq.Header.Set("Content-Type", "application/octet-stream")
	initRec := httptest.NewRecorder()

	server.handleInitiateMultipartUpload(initRec, initReq, "flow-bucket", "large-file.bin")

	if initRec.Code != http.StatusOK {
		t.Fatalf("初始化上传失败: %d", initRec.Code)
	}

	var initResult InitiateMultipartUploadResult
	xml.Unmarshal(initRec.Body.Bytes(), &initResult)
	uploadID := initResult.UploadId

	// 2. 上传分片
	partETags := make([]string, 3)
	for i := 1; i <= 3; i++ {
		content := bytes.Repeat([]byte{byte('A' + i - 1)}, 5*1024*1024) // 5MB per part
		partReq := httptest.NewRequest(http.MethodPut, "/flow-bucket/large-file.bin?uploadId="+uploadID+"&partNumber="+strconv.Itoa(i), bytes.NewReader(content))
		partRec := httptest.NewRecorder()

		server.handleUploadPart(partRec, partReq, "flow-bucket", "large-file.bin", uploadID)

		if partRec.Code != http.StatusOK {
			t.Fatalf("上传分片%d失败: %d", i, partRec.Code)
		}

		partETags[i-1] = strings.Trim(partRec.Header().Get("ETag"), `"`)
	}

	// 3. 列出分片验证
	listReq := httptest.NewRequest(http.MethodGet, "/flow-bucket/large-file.bin?uploadId="+uploadID, nil)
	listRec := httptest.NewRecorder()

	server.handleListParts(listRec, listReq, "flow-bucket", "large-file.bin", uploadID)

	if listRec.Code != http.StatusOK {
		t.Fatalf("列出分片失败: %d", listRec.Code)
	}

	var listResult ListPartsResult
	xml.Unmarshal(listRec.Body.Bytes(), &listResult)

	if len(listResult.Parts) != 3 {
		t.Errorf("分片数量错误: 期望 3, 实际 %d", len(listResult.Parts))
	}

	// 4. 完成上传
	completeXML := `<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUpload>
  <Part><PartNumber>1</PartNumber><ETag>"` + partETags[0] + `"</ETag></Part>
  <Part><PartNumber>2</PartNumber><ETag>"` + partETags[1] + `"</ETag></Part>
  <Part><PartNumber>3</PartNumber><ETag>"` + partETags[2] + `"</ETag></Part>
</CompleteMultipartUpload>`

	completeReq := httptest.NewRequest(http.MethodPost, "/flow-bucket/large-file.bin?uploadId="+uploadID, strings.NewReader(completeXML))
	completeRec := httptest.NewRecorder()

	server.handleCompleteMultipartUpload(completeRec, completeReq, "flow-bucket", "large-file.bin", uploadID)

	if completeRec.Code != http.StatusOK {
		t.Fatalf("完成上传失败: %d, 响应: %s", completeRec.Code, completeRec.Body.String())
	}

	// 5. 验证对象已创建
	obj, err := server.metadata.GetObject("flow-bucket", "large-file.bin")
	if err != nil {
		t.Fatalf("获取对象失败: %v", err)
	}
	if obj == nil {
		t.Fatal("对象应已创建")
	}

	expectedSize := int64(3 * 5 * 1024 * 1024) // 15MB
	if obj.Size != expectedSize {
		t.Errorf("对象大小错误: 期望 %d, 实际 %d", expectedSize, obj.Size)
	}

	// 6. 验证可以获取对象
	getReq := httptest.NewRequest(http.MethodGet, "/flow-bucket/large-file.bin", nil)
	getRec := httptest.NewRecorder()

	server.handleGetObject(getRec, getReq, "flow-bucket", "large-file.bin")

	if getRec.Code != http.StatusOK {
		t.Errorf("获取对象失败: %d", getRec.Code)
	}
	if int64(getRec.Body.Len()) != expectedSize {
		t.Errorf("获取的对象大小错误: %d", getRec.Body.Len())
	}
}

// TestConcurrentMultipartUpload 测试并发多部分上传
func TestConcurrentMultipartUpload(t *testing.T) {
	server, cleanup := setupMultipartTestServer(t)
	defer cleanup()

	if err := server.metadata.CreateBucket("concurrent-mp-bucket"); err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	numUploads := 5
	done := make(chan bool, numUploads)

	for i := 0; i < numUploads; i++ {
		go func(id int) {
			key := "concurrent-file-" + strconv.Itoa(id) + ".bin"

			// 初始化
			initReq := httptest.NewRequest(http.MethodPost, "/concurrent-mp-bucket/"+key+"?uploads", nil)
			initRec := httptest.NewRecorder()
			server.handleInitiateMultipartUpload(initRec, initReq, "concurrent-mp-bucket", key)

			if initRec.Code != http.StatusOK {
				t.Errorf("并发初始化失败: id=%d, status=%d", id, initRec.Code)
				done <- false
				return
			}

			var initResult InitiateMultipartUploadResult
			xml.Unmarshal(initRec.Body.Bytes(), &initResult)
			uploadID := initResult.UploadId

			// 上传分片
			content := bytes.Repeat([]byte{byte(id)}, 1024)
			partReq := httptest.NewRequest(http.MethodPut, "/concurrent-mp-bucket/"+key+"?uploadId="+uploadID+"&partNumber=1", bytes.NewReader(content))
			partRec := httptest.NewRecorder()
			server.handleUploadPart(partRec, partReq, "concurrent-mp-bucket", key, uploadID)

			if partRec.Code != http.StatusOK {
				t.Errorf("并发上传分片失败: id=%d, status=%d", id, partRec.Code)
				done <- false
				return
			}

			etag := strings.Trim(partRec.Header().Get("ETag"), `"`)

			// 完成
			completeXML := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>"` + etag + `"</ETag></Part></CompleteMultipartUpload>`
			completeReq := httptest.NewRequest(http.MethodPost, "/concurrent-mp-bucket/"+key+"?uploadId="+uploadID, strings.NewReader(completeXML))
			completeRec := httptest.NewRecorder()
			server.handleCompleteMultipartUpload(completeRec, completeReq, "concurrent-mp-bucket", key, uploadID)

			if completeRec.Code != http.StatusOK {
				t.Errorf("并发完成失败: id=%d, status=%d", id, completeRec.Code)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// 等待所有完成
	successCount := 0
	for i := 0; i < numUploads; i++ {
		if <-done {
			successCount++
		}
	}

	if successCount != numUploads {
		t.Errorf("并发上传成功数: 期望 %d, 实际 %d", numUploads, successCount)
	}
}

// TestXMLStructureSerialization 测试XML结构序列化
func TestXMLStructureSerialization(t *testing.T) {
	t.Run("InitiateMultipartUploadResult序列化", func(t *testing.T) {
		result := InitiateMultipartUploadResult{
			Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
			Bucket:   "test-bucket",
			Key:      "test-key",
			UploadId: "test-upload-id",
		}

		data, err := xml.Marshal(result)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}

		xmlStr := string(data)
		if !strings.Contains(xmlStr, "InitiateMultipartUploadResult") {
			t.Error("应包含InitiateMultipartUploadResult")
		}
		if !strings.Contains(xmlStr, "test-bucket") {
			t.Error("应包含bucket名称")
		}
	})

	t.Run("CompleteMultipartUploadResult序列化", func(t *testing.T) {
		result := CompleteMultipartUploadResult{
			Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
			Location: "/bucket/key",
			Bucket:   "test-bucket",
			Key:      "test-key",
			ETag:     `"abc123"`,
		}

		data, err := xml.Marshal(result)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}

		xmlStr := string(data)
		if !strings.Contains(xmlStr, "CompleteMultipartUploadResult") {
			t.Error("应包含CompleteMultipartUploadResult")
		}
		if !strings.Contains(xmlStr, "<ETag>") {
			t.Error("应包含ETag")
		}
	})

	t.Run("ListPartsResult序列化", func(t *testing.T) {
		result := ListPartsResult{
			Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
			Bucket:   "test-bucket",
			Key:      "test-key",
			UploadId: "test-upload-id",
			MaxParts: 1000,
			Parts: []PartInfo{
				{PartNumber: 1, ETag: `"etag1"`, Size: 1024},
				{PartNumber: 2, ETag: `"etag2"`, Size: 2048},
			},
		}

		data, err := xml.Marshal(result)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}

		xmlStr := string(data)
		if !strings.Contains(xmlStr, "ListPartsResult") {
			t.Error("应包含ListPartsResult")
		}
		if !strings.Contains(xmlStr, "<Part>") {
			t.Error("应包含Part元素")
		}
	})
}

// BenchmarkHandleUploadPart 基准测试-上传分片
func BenchmarkHandleUploadPart(b *testing.B) {
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/bench.db")
	defer metadata.Close()
	filestore, _ := storage.NewFileStore(tempDir)
	server := NewServer(metadata, filestore)

	metadata.CreateBucket("bench-bucket")

	// 创建上传
	upload := &storage.MultipartUpload{
		UploadID:    "bench-upload-id",
		Bucket:      "bench-bucket",
		Key:         "bench-file.bin",
		ContentType: "application/octet-stream",
	}
	metadata.CreateMultipartUpload(upload)

	content := bytes.Repeat([]byte("x"), 1024*1024) // 1MB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		partNum := (i % 10000) + 1
		req := httptest.NewRequest(http.MethodPut, "/bench-bucket/bench-file.bin?uploadId=bench-upload-id&partNumber="+strconv.Itoa(partNum), bytes.NewReader(content))
		rec := httptest.NewRecorder()
		server.handleUploadPart(rec, req, "bench-bucket", "bench-file.bin", "bench-upload-id")
	}
}

// BenchmarkHandleInitiateMultipartUpload 基准测试-初始化上传
func BenchmarkHandleInitiateMultipartUpload(b *testing.B) {
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := b.TempDir()
	metadata, _ := storage.NewMetadataStore(tempDir + "/bench.db")
	defer metadata.Close()
	filestore, _ := storage.NewFileStore(tempDir)
	server := NewServer(metadata, filestore)

	metadata.CreateBucket("bench-bucket")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench-file-" + strconv.Itoa(i) + ".bin"
		req := httptest.NewRequest(http.MethodPost, "/bench-bucket/"+key+"?uploads", nil)
		rec := httptest.NewRecorder()
		server.handleInitiateMultipartUpload(rec, req, "bench-bucket", key)
	}
}
