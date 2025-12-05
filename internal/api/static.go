package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// serveStatic 处理静态文件请求
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	// 静态文件目录
	staticDir := "./data/static"

	// 获取请求路径
	path := r.URL.Path

	// 如果是根路径，返回 index.html
	if path == "/" {
		s.serveFile(w, r, filepath.Join(staticDir, "index.html"))
		return
	}

	// 处理 hash 路由
	if strings.Contains(path, "#") {
		s.serveFile(w, r, filepath.Join(staticDir, "index.html"))
		return
	}

	// 处理静态资源
	if strings.HasPrefix(path, "/assets/") {
		filePath := filepath.Join(staticDir, path)
		s.serveFile(w, r, filePath)
		return
	}

	// 其他路径也返回 index.html (SPA 路由)
	s.serveFile(w, r, filepath.Join(staticDir, "index.html"))
}

// serveFile 发送文件
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, filePath string) {
	// 检查文件是否存在
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// 设置 Content-Type
	ext := filepath.Ext(filePath)
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	// 发送文件
	http.ServeFile(w, r, filePath)
}