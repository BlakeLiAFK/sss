package admin

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"sss/internal/auth"
	"sss/internal/storage"
	"sss/internal/utils"
)

// PreviewResponse 预览响应
type PreviewResponse struct {
	Type        string `json:"type"`                // text, image, video, audio, pdf, office, binary
	MimeType    string `json:"mime_type"`           // MIME 类型
	Size        int64  `json:"size"`                // 文件大小
	Content     string `json:"content,omitempty"`   // 文本内容（仅文本类型）
	URL         string `json:"url,omitempty"`       // 预签名 URL（用于直接预览）
	Previewable bool   `json:"previewable"`         // 是否可预览
	Truncated   bool   `json:"truncated,omitempty"` // 内容是否被截断
	Lines       int    `json:"lines,omitempty"`     // 文本行数
	Encoding    string `json:"encoding,omitempty"`  // 文本编码
}

// 预览大小限制
const (
	maxPreviewSize   = 1024 * 1024 // 1MB - 文本预览最大大小
	maxPreviewLines  = 1000        // 最大预览行数
	previewURLExpiry = 15          // 预签名 URL 有效期（分钟）
)

// 可预览的文本文件扩展名
var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".rst": true,
	".json": true, ".xml": true, ".yaml": true, ".yml": true, ".toml": true,
	".html": true, ".htm": true, ".css": true, ".scss": true, ".less": true,
	".js": true, ".ts": true, ".jsx": true, ".tsx": true, ".vue": true, ".svelte": true,
	".go": true, ".py": true, ".rb": true, ".php": true, ".java": true, ".kt": true,
	".c": true, ".cpp": true, ".h": true, ".hpp": true, ".cs": true,
	".rs": true, ".swift": true, ".m": true, ".mm": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".ps1": true, ".bat": true, ".cmd": true,
	".sql": true, ".graphql": true, ".gql": true,
	".dockerfile": true, ".gitignore": true, ".editorconfig": true,
	".env": true, ".ini": true, ".conf": true, ".cfg": true, ".properties": true,
	".log": true, ".csv": true, ".tsv": true,
	".makefile": true, ".cmake": true,
}

// 可预览的图片扩展名
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".svg": true, ".ico": true, ".bmp": true, ".tiff": true, ".tif": true,
}

// 可预览的视频扩展名
var videoExtensions = map[string]bool{
	".mp4": true, ".webm": true, ".ogg": true, ".ogv": true, ".mov": true,
	".m4v": true, ".avi": true, ".mkv": true,
}

// 可预览的音频扩展名
var audioExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".ogg": true, ".oga": true, ".m4a": true,
	".flac": true, ".aac": true, ".wma": true,
}

// PDF 扩展名
var pdfExtensions = map[string]bool{
	".pdf": true,
}

// previewObject 预览对象
// GET /api/admin/buckets/{bucket}/preview?key=xxx
func (h *Handler) previewObject(w http.ResponseWriter, r *http.Request, bucketName string) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, utils.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "")
		return
	}

	// 获取对象 key
	key := r.URL.Query().Get("key")
	if key == "" {
		utils.WriteErrorResponse(w, "MissingParameter", "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	// 安全检查：防止路径遍历
	if strings.Contains(key, "..") {
		utils.WriteErrorResponse(w, "InvalidParameter", "Invalid key", http.StatusBadRequest)
		return
	}

	// 获取对象元数据
	obj, err := h.metadata.GetObject(bucketName, key)
	if err != nil {
		utils.Error("get object for preview failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	if obj == nil {
		utils.WriteError(w, utils.ErrNoSuchKey, http.StatusNotFound, "")
		return
	}

	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(key))
	// 特殊处理无扩展名的文件（如 Dockerfile, Makefile 等）
	if ext == "" {
		baseName := strings.ToLower(filepath.Base(key))
		if baseName == "dockerfile" || baseName == "makefile" || baseName == "rakefile" ||
			baseName == "gemfile" || baseName == "vagrantfile" || baseName == "jenkinsfile" {
			ext = "." + baseName
		}
	}

	// 确定文件类型
	resp := PreviewResponse{
		Size:     obj.Size,
		MimeType: getMimeType(ext),
	}

	switch {
	case textExtensions[ext]:
		resp.Type = "text"
		resp.Previewable = true
		h.handleTextPreview(w, &resp, obj, bucketName, key)
		return

	case imageExtensions[ext]:
		resp.Type = "image"
		resp.Previewable = true
		resp.URL = h.generatePreviewURL(bucketName, key)

	case videoExtensions[ext]:
		resp.Type = "video"
		resp.Previewable = true
		resp.URL = h.generatePreviewURL(bucketName, key)

	case audioExtensions[ext]:
		resp.Type = "audio"
		resp.Previewable = true
		resp.URL = h.generatePreviewURL(bucketName, key)

	case pdfExtensions[ext]:
		resp.Type = "pdf"
		resp.Previewable = true
		resp.URL = h.generatePreviewURL(bucketName, key)

	default:
		resp.Type = "binary"
		resp.Previewable = false
		resp.URL = h.generatePreviewURL(bucketName, key)
	}

	utils.WriteJSONResponse(w, resp)
}

// handleTextPreview 处理文本文件预览
func (h *Handler) handleTextPreview(w http.ResponseWriter, resp *PreviewResponse, obj *storage.Object, bucket, key string) {
	// 检查文件大小
	if obj.Size > maxPreviewSize {
		// 文件过大，只读取前 1MB
		resp.Truncated = true
	}

	// 读取文件内容
	file, err := h.filestore.GetObject(obj.StoragePath)
	if err != nil {
		utils.Error("open file for preview failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	defer file.Close()

	// 读取内容（限制大小）
	readSize := obj.Size
	if readSize > maxPreviewSize {
		readSize = maxPreviewSize
	}

	content := make([]byte, readSize)
	n, err := io.ReadFull(file, content)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		utils.Error("read file for preview failed", "error", err)
		utils.WriteError(w, utils.ErrInternalError, http.StatusInternalServerError, "")
		return
	}
	content = content[:n]

	// 检测编码（简单检测是否为 UTF-8）
	resp.Encoding = "utf-8"
	if !isValidUTF8(content) {
		resp.Encoding = "binary"
		resp.Type = "binary"
		resp.Previewable = false
		resp.Content = ""
		resp.URL = h.generatePreviewURL(bucket, key)
		utils.WriteJSONResponse(w, resp)
		return
	}

	// 转换为字符串并限制行数
	text := string(content)
	lines := strings.Split(text, "\n")
	resp.Lines = len(lines)

	if len(lines) > maxPreviewLines {
		lines = lines[:maxPreviewLines]
		resp.Truncated = true
	}

	resp.Content = strings.Join(lines, "\n")
	utils.WriteJSONResponse(w, resp)
}

// generatePreviewURL 生成预览用的预签名 URL
func (h *Handler) generatePreviewURL(bucket, key string) string {
	opts := &auth.PresignOptions{
		Expires: time.Duration(previewURLExpiry) * time.Minute,
	}
	return auth.GeneratePresignedURLWithOptions("GET", bucket, key, opts)
}

// getMimeType 根据扩展名获取 MIME 类型
func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		// 文本
		".txt":      "text/plain",
		".md":       "text/markdown",
		".markdown": "text/markdown",
		".json":     "application/json",
		".xml":      "application/xml",
		".yaml":     "text/yaml",
		".yml":      "text/yaml",
		".html":     "text/html",
		".htm":      "text/html",
		".css":      "text/css",
		".js":       "application/javascript",
		".ts":       "application/typescript",
		".go":       "text/x-go",
		".py":       "text/x-python",
		".rb":       "text/x-ruby",
		".php":      "text/x-php",
		".java":     "text/x-java",
		".c":        "text/x-c",
		".cpp":      "text/x-c++",
		".h":        "text/x-c",
		".hpp":      "text/x-c++",
		".cs":       "text/x-csharp",
		".rs":       "text/x-rust",
		".swift":    "text/x-swift",
		".sql":      "text/x-sql",
		".sh":       "text/x-shellscript",
		".bash":     "text/x-shellscript",
		".log":      "text/plain",
		".csv":      "text/csv",

		// 图片
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".bmp":  "image/bmp",

		// 视频
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".ogg":  "video/ogg",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",

		// 音频
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".m4a":  "audio/mp4",
		".flac": "audio/flac",
		".aac":  "audio/aac",

		// PDF
		".pdf": "application/pdf",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// isValidUTF8 检查字节是否为有效的 UTF-8 编码
func isValidUTF8(data []byte) bool {
	// 检查是否包含 NULL 字节（二进制文件通常包含）
	for _, b := range data {
		if b == 0 {
			return false
		}
	}

	// 使用标准库检查 UTF-8
	for i := 0; i < len(data); {
		r, size := decodeRune(data[i:])
		if r == 0xFFFD && size == 1 {
			// 无效的 UTF-8 序列
			return false
		}
		i += size
	}
	return true
}

// decodeRune 解码 UTF-8 字节序列
func decodeRune(p []byte) (r rune, size int) {
	if len(p) == 0 {
		return 0xFFFD, 0
	}

	c := p[0]
	if c < 0x80 {
		return rune(c), 1
	}

	// 多字节序列
	var n int
	switch {
	case c < 0xC0:
		return 0xFFFD, 1 // 无效的起始字节
	case c < 0xE0:
		n = 2
	case c < 0xF0:
		n = 3
	case c < 0xF8:
		n = 4
	default:
		return 0xFFFD, 1
	}

	if len(p) < n {
		return 0xFFFD, 1
	}

	// 验证后续字节
	for i := 1; i < n; i++ {
		if p[i]&0xC0 != 0x80 {
			return 0xFFFD, 1
		}
	}

	// 解码
	switch n {
	case 2:
		r = rune(p[0]&0x1F)<<6 | rune(p[1]&0x3F)
	case 3:
		r = rune(p[0]&0x0F)<<12 | rune(p[1]&0x3F)<<6 | rune(p[2]&0x3F)
	case 4:
		r = rune(p[0]&0x07)<<18 | rune(p[1]&0x3F)<<12 | rune(p[2]&0x3F)<<6 | rune(p[3]&0x3F)
	}

	return r, n
}
