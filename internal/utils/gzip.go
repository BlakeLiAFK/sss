package utils

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzip writer 池，减少内存分配
var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
		return w
	},
}

// gzipResponseWriter 包装 http.ResponseWriter 以支持 gzip 压缩
type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter *gzip.Writer
}

func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	return g.gzipWriter.Write(data)
}

// GzipMiddleware 返回一个 gzip 压缩中间件
// 只对文本类型的响应进行压缩
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查客户端是否支持 gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// 检查请求路径，只对静态资源和 API 响应压缩
		path := r.URL.Path
		shouldCompress := strings.HasPrefix(path, "/assets/") ||
			strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".html") ||
			strings.HasSuffix(path, ".json") ||
			strings.HasSuffix(path, ".svg") ||
			strings.HasPrefix(path, "/api/")

		if !shouldCompress {
			next.ServeHTTP(w, r)
			return
		}

		// 从池中获取 gzip writer
		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			gz.Close()
			gzipPool.Put(gz)
		}()

		// 设置响应头
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		// 删除 Content-Length，因为压缩后长度会变化
		w.Header().Del("Content-Length")

		// 使用 gzip writer 包装响应
		gzipWriter := &gzipResponseWriter{
			ResponseWriter: w,
			gzipWriter:     gz,
		}

		next.ServeHTTP(gzipWriter, r)
	})
}

// GzipHandler 包装一个 http.Handler 并添加 gzip 支持
func GzipHandler(h http.Handler) http.Handler {
	return GzipMiddleware(h)
}

// 确保 gzipResponseWriter 实现了必要的接口
var _ http.ResponseWriter = (*gzipResponseWriter)(nil)
var _ io.Writer = (*gzipResponseWriter)(nil)
