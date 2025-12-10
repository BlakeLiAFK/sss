package utils

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGzipMiddleware_WithGzipSupport 测试支持 gzip 的请求
func TestGzipMiddleware_WithGzipSupport(t *testing.T) {
	testCases := []struct {
		name           string
		path           string
		shouldCompress bool
	}{
		{"JS文件", "/assets/app.js", true},
		{"CSS文件", "/assets/style.css", true},
		{"HTML文件", "/index.html", true},
		{"JSON API", "/api/test", true},
		{"SVG文件", "/icon.svg", true},
		{"PNG图片", "/image.png", false},      // 不压缩
		{"普通路径", "/some/path", false},      // 不压缩
		{"无后缀路径", "/download", false},     // 不压缩
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试处理器
			testContent := "This is test content for gzip compression testing."
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(testContent))
			})

			// 包装 gzip 中间件
			wrapped := GzipMiddleware(handler)

			// 创建请求
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Accept-Encoding", "gzip, deflate")
			rec := httptest.NewRecorder()

			// 执行请求
			wrapped.ServeHTTP(rec, req)

			// 检查结果
			if tc.shouldCompress {
				if rec.Header().Get("Content-Encoding") != "gzip" {
					t.Errorf("期望 Content-Encoding: gzip, 实际: %s", rec.Header().Get("Content-Encoding"))
				}

				// 解压并验证内容
				reader, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
				if err != nil {
					t.Fatalf("创建 gzip reader 失败: %v", err)
				}
				defer reader.Close()

				decompressed, err := io.ReadAll(reader)
				if err != nil {
					t.Fatalf("解压失败: %v", err)
				}

				if string(decompressed) != testContent {
					t.Errorf("解压内容不匹配: got %q, want %q", string(decompressed), testContent)
				}
			} else {
				if rec.Header().Get("Content-Encoding") == "gzip" {
					t.Errorf("不应该压缩该路径: %s", tc.path)
				}

				// 验证原始内容
				if rec.Body.String() != testContent {
					t.Errorf("内容不匹配: got %q, want %q", rec.Body.String(), testContent)
				}
			}
		})
	}
}

// TestGzipMiddleware_WithoutGzipSupport 测试不支持 gzip 的请求
func TestGzipMiddleware_WithoutGzipSupport(t *testing.T) {
	testContent := "This is test content."
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testContent))
	})

	wrapped := GzipMiddleware(handler)

	// 创建不支持 gzip 的请求
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	// 不设置 Accept-Encoding
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// 不应该压缩
	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("不支持 gzip 的客户端不应该收到压缩响应")
	}

	// 验证原始内容
	if rec.Body.String() != testContent {
		t.Errorf("内容不匹配: got %q, want %q", rec.Body.String(), testContent)
	}
}

// TestGzipHandler 测试 GzipHandler 包装函数
func TestGzipHandler(t *testing.T) {
	testContent := "Test GzipHandler"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testContent))
	})

	wrapped := GzipHandler(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("GzipHandler 应该压缩响应")
	}
}

// TestGzipResponseWriter_Write 测试 gzipResponseWriter 的 Write 方法
func TestGzipResponseWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	rec := httptest.NewRecorder()
	gzw := &gzipResponseWriter{
		ResponseWriter: rec,
		gzipWriter:     gz,
	}

	testData := []byte("Hello, World!")
	n, err := gzw.Write(testData)
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	if n != len(testData) {
		t.Errorf("写入字节数错误: got %d, want %d", n, len(testData))
	}

	// 关闭 gzip writer 以刷新缓冲区
	gz.Close()
}

// TestGzipMiddleware_VaryHeader 测试 Vary 头设置
func TestGzipMiddleware_VaryHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})

	wrapped := GzipMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// 检查 Vary 头
	if rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("Vary 头错误: got %q, want %q", rec.Header().Get("Vary"), "Accept-Encoding")
	}
}

// BenchmarkGzipMiddleware 基准测试 gzip 中间件
func BenchmarkGzipMiddleware(b *testing.B) {
	testContent := bytes.Repeat([]byte("benchmark test content "), 100)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(testContent)
	})

	wrapped := GzipMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/benchmark", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}
