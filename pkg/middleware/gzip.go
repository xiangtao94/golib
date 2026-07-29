package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GzipMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		addVary(ctx.Writer.Header(), "Accept-Encoding")
		if ctx.Request.Method == http.MethodHead || !acceptsGzip(ctx.GetHeader("Accept-Encoding")) {
			ctx.Next()
			return
		}

		writer := &gzipResponseWriter{
			ResponseWriter: ctx.Writer,
			status:         http.StatusOK,
			requestMethod:  ctx.Request.Method,
		}
		ctx.Writer = writer
		defer writer.close()
		ctx.Next()
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	gzipWriter    *gzip.Writer
	status        int
	size          int
	wroteHeader   bool
	compressed    bool
	requestMethod string
}

func (writer *gzipResponseWriter) WriteHeader(status int) {
	if writer.wroteHeader {
		return
	}
	writer.status = status
}

func (writer *gzipResponseWriter) WriteHeaderNow() {
	if writer.wroteHeader {
		return
	}
	writer.decideCompression()
	writer.ResponseWriter.WriteHeader(writer.status)
	writer.wroteHeader = true
}

func (writer *gzipResponseWriter) Write(data []byte) (int, error) {
	if writer.Header().Get("Content-Type") == "" {
		writer.Header().Set("Content-Type", http.DetectContentType(data))
	}
	writer.WriteHeaderNow()
	var (
		written int
		err     error
	)
	if writer.compressed {
		written, err = writer.gzipWriter.Write(data)
	} else {
		written, err = writer.ResponseWriter.Write(data)
	}
	writer.size += written
	return written, err
}

func (writer *gzipResponseWriter) WriteString(data string) (int, error) {
	return writer.Write([]byte(data))
}

func (writer *gzipResponseWriter) Flush() {
	writer.WriteHeaderNow()
	if writer.gzipWriter != nil {
		_ = writer.gzipWriter.Flush()
	}
	writer.ResponseWriter.Flush()
}

func (writer *gzipResponseWriter) Status() int { return writer.status }
func (writer *gzipResponseWriter) Size() int   { return writer.size }
func (writer *gzipResponseWriter) Written() bool {
	return writer.wroteHeader
}

func (writer *gzipResponseWriter) decideCompression() {
	contentType := strings.ToLower(writer.Header().Get("Content-Type"))
	noBodyStatus := writer.status < 200 || writer.status == http.StatusNoContent || writer.status == http.StatusNotModified
	if writer.requestMethod == http.MethodHead || noBodyStatus || strings.HasPrefix(contentType, "text/event-stream") {
		writer.Header().Del("Content-Encoding")
		return
	}
	writer.Header().Del("Content-Length")
	writer.Header().Set("Content-Encoding", "gzip")
	writer.gzipWriter = gzip.NewWriter(writer.ResponseWriter)
	writer.compressed = true
}

func (writer *gzipResponseWriter) close() {
	if !writer.wroteHeader {
		writer.WriteHeaderNow()
	}
	if writer.gzipWriter != nil {
		_ = writer.gzipWriter.Close()
	}
}

func acceptsGzip(header string) bool {
	for value := range strings.SplitSeq(header, ",") {
		parts := strings.Split(strings.TrimSpace(value), ";")
		if !strings.EqualFold(parts[0], "gzip") {
			continue
		}
		for _, parameter := range parts[1:] {
			if strings.TrimSpace(parameter) == "q=0" {
				return false
			}
		}
		return true
	}
	return false
}

func addVary(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for item := range strings.SplitSeq(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(item), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

var _ io.Writer = (*gzipResponseWriter)(nil)
