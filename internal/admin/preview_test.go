package admin

import (
	"encoding/json"
	"testing"
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
