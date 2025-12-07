package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// TestGetMimeType 测试MIME类型检测
func TestGetMimeType(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected string
	}{
		// 文本类型
		{"TXT文件", ".txt", "text/plain"},
		{"Markdown文件", ".md", "text/markdown"},
		{"JSON文件", ".json", "application/json"},
		{"XML文件", ".xml", "application/xml"},
		{"HTML文件", ".html", "text/html"},
		{"CSS文件", ".css", "text/css"},
		{"JS文件", ".js", "application/javascript"},
		{"Go源文件", ".go", "text/x-go"},
		{"Python源文件", ".py", "text/x-python"},

		// 图片类型
		{"PNG图片", ".png", "image/png"},
		{"JPEG图片", ".jpg", "image/jpeg"},
		{"GIF图片", ".gif", "image/gif"},
		{"WebP图片", ".webp", "image/webp"},
		{"SVG图片", ".svg", "image/svg+xml"},

		// 视频类型
		{"MP4视频", ".mp4", "video/mp4"},
		{"WebM视频", ".webm", "video/webm"},
		{"AVI视频", ".avi", "video/x-msvideo"},
		{"MOV视频", ".mov", "video/quicktime"},

		// 音频类型
		{"MP3音频", ".mp3", "audio/mpeg"},
		{"WAV音频", ".wav", "audio/wav"},
		// 注意: .ogg 在 mimeTypes 中定义为 video/ogg (OGG 是容器格式)

		// PDF
		{"PDF文件", ".pdf", "application/pdf"},

		// 未知类型
		{"未知扩展名", ".xyz", "application/octet-stream"},
		{"空扩展名", "", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMimeType(tt.ext)
			if result != tt.expected {
				t.Errorf("getMimeType(%q) = %q, want %q", tt.ext, result, tt.expected)
			}
		})
	}
}

// TestIsValidUTF8 测试UTF-8验证
func TestIsValidUTF8(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{"空字节", []byte{}, true},
		{"ASCII文本", []byte("Hello, World!"), true},
		{"中文文本", []byte("你好世界"), true},
		{"日文文本", []byte("こんにちは"), true},
		{"混合文本", []byte("Hello 你好 123"), true},
		{"带换行符", []byte("line1\nline2\r\nline3"), true},
		{"带制表符", []byte("col1\tcol2\tcol3"), true},

		// 无效UTF-8序列
		{"无效UTF-8-0x80", []byte{0x80}, false},
		{"无效UTF-8-0xFF", []byte{0xFF}, false},
		{"无效UTF-8序列", []byte{0xC0, 0xC0}, false},
		{"截断的UTF-8", []byte{0xE4, 0xB8}, false}, // 只有2字节，需要3字节

		// 边界情况
		{"单字节最大", []byte{0x7F}, true},
		{"二字节序列", []byte{0xC2, 0x80}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUTF8(tt.input)
			if result != tt.expected {
				t.Errorf("isValidUTF8(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestDecodeRune 测试UTF-8解码
func TestDecodeRune(t *testing.T) {
	// 0xFFFD (65533) 是 Unicode 替换字符，用于表示无效的 UTF-8 序列
	const runeError = 0xFFFD
	tests := []struct {
		name         string
		input        []byte
		expectedRune rune
		expectedSize int
	}{
		{"ASCII字符a", []byte("abc"), 'a', 1},
		{"ASCII字符Z", []byte("Z"), 'Z', 1},
		{"中文字符", []byte("你好"), 0x4F60, 3},
		{"空输入", []byte{}, runeError, 0},
		{"无效起始字节", []byte{0x80}, runeError, 1},
		{"截断序列", []byte{0xE4, 0xB8}, runeError, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, size := decodeRune(tt.input)
			if r != tt.expectedRune || size != tt.expectedSize {
				t.Errorf("decodeRune(%v) = (%d, %d), want (%d, %d)",
					tt.input, r, size, tt.expectedRune, tt.expectedSize)
			}
		})
	}
}

// TestTextExtensions 测试文本扩展名映射
func TestTextExtensions(t *testing.T) {
	textExts := []string{
		".txt", ".md", ".markdown", ".json", ".xml", ".yaml", ".yml",
		".html", ".htm", ".css", ".js", ".ts", ".go", ".py", ".rb",
		".php", ".java", ".c", ".cpp", ".h", ".sh", ".bash", ".zsh",
		".sql", ".log", ".ini", ".cfg", ".conf", ".env", ".gitignore",
		".dockerfile", ".makefile",
	}

	for _, ext := range textExts {
		t.Run(ext, func(t *testing.T) {
			if !textExtensions[ext] {
				t.Errorf("textExtensions[%q] should be true", ext)
			}
		})
	}
}

// TestImageExtensions 测试图片扩展名映射
func TestImageExtensions(t *testing.T) {
	imageExts := []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".ico", ".svg", ".tiff", ".tif",
	}

	for _, ext := range imageExts {
		t.Run(ext, func(t *testing.T) {
			if !imageExtensions[ext] {
				t.Errorf("imageExtensions[%q] should be true", ext)
			}
		})
	}
}

// TestVideoExtensions 测试视频扩展名映射
func TestVideoExtensions(t *testing.T) {
	// 注意: 实现中只支持 mp4, webm, ogg, ogv, mov, m4v, avi, mkv
	// 不支持 wmv, flv
	videoExts := []string{
		".mp4", ".webm", ".ogg", ".ogv", ".avi", ".mov", ".mkv", ".m4v",
	}

	for _, ext := range videoExts {
		t.Run(ext, func(t *testing.T) {
			if !videoExtensions[ext] {
				t.Errorf("videoExtensions[%q] should be true", ext)
			}
		})
	}
}

// TestAudioExtensions 测试音频扩展名映射
func TestAudioExtensions(t *testing.T) {
	audioExts := []string{
		".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a", ".wma",
	}

	for _, ext := range audioExts {
		t.Run(ext, func(t *testing.T) {
			if !audioExtensions[ext] {
				t.Errorf("audioExtensions[%q] should be true", ext)
			}
		})
	}
}

// TestPDFExtensions 测试PDF扩展名映射
func TestPDFExtensions(t *testing.T) {
	if !pdfExtensions[".pdf"] {
		t.Errorf("pdfExtensions[.pdf] should be true")
	}
}

// TestPreviewResponse 测试预览响应结构
func TestPreviewResponse(t *testing.T) {
	resp := PreviewResponse{
		Type:        "text",
		MimeType:    "text/plain",
		Size:        100,
		Content:     "Hello World",
		Previewable: true,
		Truncated:   false,
		Lines:       1,
		Encoding:    "utf-8",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("JSON序列化失败: %v", err)
	}

	var decoded PreviewResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON反序列化失败: %v", err)
	}

	if decoded.Type != resp.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, resp.Type)
	}
	if decoded.MimeType != resp.MimeType {
		t.Errorf("MimeType = %q, want %q", decoded.MimeType, resp.MimeType)
	}
	if decoded.Content != resp.Content {
		t.Errorf("Content = %q, want %q", decoded.Content, resp.Content)
	}
	if decoded.Previewable != resp.Previewable {
		t.Errorf("Previewable = %v, want %v", decoded.Previewable, resp.Previewable)
	}
	if decoded.Encoding != resp.Encoding {
		t.Errorf("Encoding = %q, want %q", decoded.Encoding, resp.Encoding)
	}
}

// TestPreviewResponseOmitEmpty 测试预览响应的omitempty标签
func TestPreviewResponseOmitEmpty(t *testing.T) {
	// 测试不含可选字段时JSON不包含它们
	resp := PreviewResponse{
		Type:        "image",
		MimeType:    "image/png",
		Size:        1024,
		Previewable: true,
		URL:         "http://example.com/image.png",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("JSON序列化失败: %v", err)
	}

	// 检查不包含Content字段（因为为空）
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("JSON解析失败: %v", err)
	}

	if _, ok := m["content"]; ok {
		t.Errorf("空content不应该出现在JSON中")
	}
	if _, ok := m["truncated"]; ok {
		t.Errorf("false truncated不应该出现在JSON中")
	}
}

// TestPreviewConstants 测试预览相关常量
func TestPreviewConstants(t *testing.T) {
	// 验证常量定义合理
	if maxPreviewSize <= 0 {
		t.Errorf("maxPreviewSize 应该大于0，当前值: %d", maxPreviewSize)
	}
	if maxPreviewLines <= 0 {
		t.Errorf("maxPreviewLines 应该大于0，当前值: %d", maxPreviewLines)
	}
	if previewURLExpiry <= 0 {
		t.Errorf("previewURLExpiry 应该大于0，当前值: %d", previewURLExpiry)
	}

	// 验证合理范围
	if maxPreviewSize > 10*1024*1024 { // 不超过10MB
		t.Errorf("maxPreviewSize 不应超过10MB，当前值: %d", maxPreviewSize)
	}
	if maxPreviewLines > 100000 { // 不超过10万行
		t.Errorf("maxPreviewLines 不应超过100000，当前值: %d", maxPreviewLines)
	}
}

// TestFileTypeDetection 测试文件类型检测逻辑
func TestFileTypeDetection(t *testing.T) {
	tests := []struct {
		ext          string
		expectedType string
	}{
		// 文本文件
		{".txt", "text"},
		{".go", "text"},
		{".py", "text"},
		{".json", "text"},

		// 图片文件
		{".png", "image"},
		{".jpg", "image"},
		{".gif", "image"},

		// 视频文件
		{".mp4", "video"},
		{".webm", "video"},

		// 音频文件
		{".mp3", "audio"},
		{".wav", "audio"},

		// PDF
		{".pdf", "pdf"},

		// 未知类型
		{".xyz", "binary"},
		{".exe", "binary"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			var detectedType string

			switch {
			case textExtensions[tt.ext]:
				detectedType = "text"
			case imageExtensions[tt.ext]:
				detectedType = "image"
			case videoExtensions[tt.ext]:
				detectedType = "video"
			case audioExtensions[tt.ext]:
				detectedType = "audio"
			case pdfExtensions[tt.ext]:
				detectedType = "pdf"
			default:
				detectedType = "binary"
			}

			if detectedType != tt.expectedType {
				t.Errorf("文件扩展名 %q 检测类型为 %q, 期望 %q",
					tt.ext, detectedType, tt.expectedType)
			}
		})
	}
}

// TestMimeTypeCompleteness 测试MIME类型映射完整性
func TestMimeTypeCompleteness(t *testing.T) {
	// 确保常见扩展名都有MIME类型映射
	requiredMappings := map[string]bool{
		".txt":  true,
		".html": true,
		".css":  true,
		".js":   true,
		".json": true,
		".xml":  true,
		".png":  true,
		".jpg":  true,
		".gif":  true,
		".mp4":  true,
		".mp3":  true,
		".pdf":  true,
	}

	for ext := range requiredMappings {
		mime := getMimeType(ext)
		if mime == "application/octet-stream" {
			t.Errorf("扩展名 %q 应该有具体的MIME类型，而不是 application/octet-stream", ext)
		}
	}
}

// BenchmarkGetMimeType 性能测试
func BenchmarkGetMimeType(b *testing.B) {
	extensions := []string{".txt", ".png", ".mp4", ".pdf", ".xyz"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ext := range extensions {
			getMimeType(ext)
		}
	}
}

// BenchmarkIsValidUTF8 性能测试
func BenchmarkIsValidUTF8(b *testing.B) {
	// 模拟1KB的UTF-8文本
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte('a' + i%26)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isValidUTF8(content)
	}
}

// ============================================================================
// HTTP 处理器测试
// ============================================================================

// TestPreviewObject_HTTP 测试预览对象HTTP处理器
func TestPreviewObject_HTTP(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶和文本文件
	bucketName := "preview-test-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	// 创建文本文件
	textContent := []byte("Hello World\nLine 2\nLine 3")
	storagePath, etag, _ := handler.filestore.PutObject(bucketName, "test.txt", bytes.NewReader(textContent), int64(len(textContent)))
	obj := &storage.Object{
		Bucket:      bucketName,
		Key:         "test.txt",
		Size:        int64(len(textContent)),
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	handler.metadata.PutObject(obj)

	// 创建图片文件（空文件用于类型检测测试）
	imgContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	imgPath, imgEtag, _ := handler.filestore.PutObject(bucketName, "image.png", bytes.NewReader(imgContent), int64(len(imgContent)))
	imgObj := &storage.Object{
		Bucket:      bucketName,
		Key:         "image.png",
		Size:        int64(len(imgContent)),
		ETag:        imgEtag,
		ContentType: "image/png",
		StoragePath: imgPath,
	}
	handler.metadata.PutObject(imgObj)

	t.Run("预览文本文件成功", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=test.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp PreviewResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Type != "text" {
			t.Errorf("Type 错误: 期望 text, 实际 %s", resp.Type)
		}
		if !resp.Previewable {
			t.Error("Previewable 应该为 true")
		}
		if resp.Content == "" {
			t.Error("Content 不应为空")
		}
	})

	t.Run("预览图片文件返回URL", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=image.png", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp PreviewResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Type != "image" {
			t.Errorf("Type 错误: 期望 image, 实际 %s", resp.Type)
		}
		if resp.URL == "" {
			t.Error("URL 不应为空")
		}
	})

	t.Run("缺少key参数返回400", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历攻击返回400", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=../../../etc/passwd", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("不存在的对象返回404", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=nonexistent.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("方法限制-只允许GET", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/preview?key=test.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// TestPreviewSpecialFiles 测试特殊文件的预览
func TestPreviewSpecialFiles(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	bucketName := "preview-special-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	t.Run("预览Dockerfile", func(t *testing.T) {
		// 创建 Dockerfile
		dockerContent := []byte("FROM golang:1.21\nWORKDIR /app\nCOPY . .\nRUN go build")
		storagePath, etag, _ := handler.filestore.PutObject(bucketName, "Dockerfile", bytes.NewReader(dockerContent), int64(len(dockerContent)))
		obj := &storage.Object{
			Bucket:      bucketName,
			Key:         "Dockerfile",
			Size:        int64(len(dockerContent)),
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		}
		handler.metadata.PutObject(obj)

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=Dockerfile", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp PreviewResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Type != "text" {
			t.Errorf("Dockerfile应该识别为文本: 实际 %s", resp.Type)
		}
	})

	t.Run("预览PDF文件返回URL", func(t *testing.T) {
		// 创建假PDF（只是测试类型检测）
		pdfContent := []byte("%PDF-1.4 fake pdf")
		storagePath, etag, _ := handler.filestore.PutObject(bucketName, "doc.pdf", bytes.NewReader(pdfContent), int64(len(pdfContent)))
		obj := &storage.Object{
			Bucket:      bucketName,
			Key:         "doc.pdf",
			Size:        int64(len(pdfContent)),
			ETag:        etag,
			ContentType: "application/pdf",
			StoragePath: storagePath,
		}
		handler.metadata.PutObject(obj)

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=doc.pdf", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp PreviewResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Type != "pdf" {
			t.Errorf("PDF类型错误: 期望 pdf, 实际 %s", resp.Type)
		}
	})

	t.Run("预览未知类型返回binary", func(t *testing.T) {
		// 创建未知类型文件
		binContent := []byte{0x00, 0x01, 0x02, 0x03}
		storagePath, etag, _ := handler.filestore.PutObject(bucketName, "data.xyz", bytes.NewReader(binContent), int64(len(binContent)))
		obj := &storage.Object{
			Bucket:      bucketName,
			Key:         "data.xyz",
			Size:        int64(len(binContent)),
			ETag:        etag,
			ContentType: "application/octet-stream",
			StoragePath: storagePath,
		}
		handler.metadata.PutObject(obj)

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=data.xyz", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.previewObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp PreviewResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Type != "binary" {
			t.Errorf("未知类型应该返回binary: 实际 %s", resp.Type)
		}
		if resp.Previewable {
			t.Error("binary类型不应该是Previewable")
		}
	})
}

// TestPreviewBinaryTextFile 测试包含二进制内容的"文本"文件
func TestPreviewBinaryTextFile(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	bucketName := "preview-binary-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	// 创建包含NULL字节的txt文件（实际是二进制文件）
	binaryContent := []byte("Hello\x00World")
	storagePath, etag, _ := handler.filestore.PutObject(bucketName, "binary.txt", bytes.NewReader(binaryContent), int64(len(binaryContent)))
	obj := &storage.Object{
		Bucket:      bucketName,
		Key:         "binary.txt",
		Size:        int64(len(binaryContent)),
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	handler.metadata.PutObject(obj)

	token := sessionStore.CreateSession()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key=binary.txt", nil)
	req.Header.Set("X-Admin-Token", token)
	rec := httptest.NewRecorder()

	handler.previewObject(rec, req, bucketName)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
	}

	var resp PreviewResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	// 因为包含NULL字节，应该被识别为binary
	if resp.Type != "binary" {
		t.Errorf("包含NULL字节的文件应该识别为binary: 实际 %s", resp.Type)
	}
}

// TestPreviewVideoAudio 测试视频和音频文件预览
func TestPreviewVideoAudio(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	bucketName := "preview-media-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	testCases := []struct {
		key          string
		expectedType string
	}{
		{"video.mp4", "video"},
		{"video.webm", "video"},
		{"audio.mp3", "audio"},
		{"audio.wav", "audio"},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			// 创建文件
			content := []byte("fake media content")
			storagePath, etag, _ := handler.filestore.PutObject(bucketName, tc.key, bytes.NewReader(content), int64(len(content)))
			obj := &storage.Object{
				Bucket:      bucketName,
				Key:         tc.key,
				Size:        int64(len(content)),
				ETag:        etag,
				ContentType: "application/octet-stream",
				StoragePath: storagePath,
			}
			handler.metadata.PutObject(obj)

			token := sessionStore.CreateSession()
			req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/preview?key="+tc.key, nil)
			req.Header.Set("X-Admin-Token", token)
			rec := httptest.NewRecorder()

			handler.previewObject(rec, req, bucketName)

			if rec.Code != http.StatusOK {
				t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
			}

			var resp PreviewResponse
			json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp.Type != tc.expectedType {
				t.Errorf("类型错误: 期望 %s, 实际 %s", tc.expectedType, resp.Type)
			}
			if resp.URL == "" {
				t.Error("媒体文件应该返回预签名URL")
			}
		})
	}
}

// TestGeneratePreviewURL 测试预览URL生成
func TestGeneratePreviewURL(t *testing.T) {
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("error")
	}

	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	url := handler.generatePreviewURL("test-bucket", "test-key.txt")
	if url == "" {
		t.Error("生成的预览URL不应为空")
	}
}
