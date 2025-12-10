package api

import (
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// 静态文件系统（由 embed.go 或 embed_dev.go 初始化）
var staticFS fs.FS
var useEmbed bool

// serveStatic 处理静态文件请求
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	// 获取请求路径
	path := r.URL.Path

	// 如果是根路径，返回 index.html
	if path == "/" {
		s.serveStaticFile(w, r, "index.html")
		return
	}

	// 处理 hash 路由
	if strings.Contains(path, "#") {
		s.serveStaticFile(w, r, "index.html")
		return
	}

	// 处理静态资源（去掉开头的斜杠）
	if strings.HasPrefix(path, "/assets/") {
		s.serveStaticFile(w, r, path[1:]) // 去掉开头的 /
		return
	}

	// 处理根目录静态文件（favicon.svg, robots.txt 等）
	if strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".txt") {
		s.serveStaticFile(w, r, path[1:]) // 去掉开头的 /
		return
	}

	// 其他路径也返回 index.html (SPA 路由)
	s.serveStaticFile(w, r, "index.html")
}

// serveStaticFile 从文件系统或嵌入文件发送文件
func (s *Server) serveStaticFile(w http.ResponseWriter, r *http.Request, name string) {
	// 打开文件
	f, err := staticFS.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	// 获取文件信息
	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 如果是目录，尝试 index.html
	if stat.IsDir() {
		indexPath := filepath.Join(name, "index.html")
		f2, err := staticFS.Open(indexPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f2.Close()
		f = f2
		name = indexPath
	}

	// 设置 Content-Type
	ext := filepath.Ext(name)
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	}

	// 设置缓存头（assets 目录使用强缓存）
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}

	// 直接复制内容（让 gzip 中间件处理压缩）
	// 注意：不设置 Content-Length，让 gzip 中间件或 chunked encoding 处理
	if _, err := io.Copy(w, f); err != nil {
		// 客户端可能已断开连接，忽略错误
		return
	}
}

// IsEmbedMode 返回是否使用嵌入模式
func IsEmbedMode() bool {
	return useEmbed
}
