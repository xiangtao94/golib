package middleware

import (
	"compress/gzip"
	"io"
	"net/http"

	"strings"

	"github.com/gin-gonic/gin"
)

// GzipMiddleware 是一个中间件，用于 gzip 压缩响应数据
func GzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			// 如果客户端不支持 gzip，则直接调用下一个处理器
			c.Next()
			return
		}

		// 设置响应头，告知客户端采用 gzip 压缩
		c.Header("Content-Encoding", "gzip")

		// 创建一个 gzip.Writer
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close() // 确保在响应结束时关闭 gzip.Writer

		// 包装 ResponseWriter
		c.Writer = &gzipResponseWriter{ResponseWriter: c.Writer, Writer: gz}

		c.Next()
	}
}

// gzipResponseWriter 包装了 gin.ResponseWriter 和 gzip.Writer
type gzipResponseWriter struct {
	gin.ResponseWriter
	io.Writer
}

// Write 方法用于压缩并写出数据
func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return w.Writer.Write(b)
}
