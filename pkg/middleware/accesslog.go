package middleware

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xiangtao94/golib/pkg/zlog"
)

const (
	_defaultPrintRequestLen  = -1
	_defaultPrintResponseLen = -1
)

// BodySanitizer converts a bounded body prefix into a safe log value. Body
// capture is disabled unless both a positive limit and a sanitizer are set.
type BodySanitizer func(contentType string, body []byte) string

type boundedCapture struct {
	body      bytes.Buffer
	limit     int
	total     int
	truncated bool
}

func newBoundedCapture(limit int) *boundedCapture {
	if limit < 0 {
		limit = 0
	}
	return &boundedCapture{limit: limit}
}

func (c *boundedCapture) Write(data []byte) (int, error) {
	originalLen := len(data)
	c.total += originalLen

	remaining := c.limit - c.body.Len()
	if remaining > 0 {
		if len(data) > remaining {
			_, _ = c.body.Write(data[:remaining])
		} else {
			_, _ = c.body.Write(data)
		}
	}
	c.truncated = c.total > c.limit
	return originalLen, nil
}

func (c *boundedCapture) Bytes() []byte {
	if c == nil {
		return nil
	}
	return c.body.Bytes()
}

func (c *boundedCapture) String() string {
	if c == nil {
		return ""
	}
	return c.body.String()
}

func (c *boundedCapture) Truncated() bool {
	return c != nil && c.truncated
}

type teeReadCloser struct {
	io.Reader
	io.Closer
}

func captureRequestBody(body io.ReadCloser, limit int) (io.ReadCloser, *boundedCapture) {
	capture := newBoundedCapture(limit)
	return &teeReadCloser{
		Reader: io.TeeReader(body, capture),
		Closer: body,
	}, capture
}

type customRespWriter struct {
	gin.ResponseWriter
	body *boundedCapture
}

func (w *customRespWriter) WriteString(value string) (int, error) {
	if w.body != nil {
		_, _ = w.body.Write([]byte(value))
	}
	return w.ResponseWriter.WriteString(value)
}

func (w *customRespWriter) Write(data []byte) (int, error) {
	if w.body != nil {
		_, _ = w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

type AccessLoggerConfig struct {
	SkipPaths    []string `yaml:"skipPaths"`
	PrintHeaders []string `yaml:"printHeaders"`

	// A body is captured only when the corresponding limit is positive and a
	// sanitizer is provided. Capture is always limited to this prefix length.
	MaxReqBodyLen         int           `yaml:"maxReqBodyLen"`
	MaxRespBodyLen        int           `yaml:"maxRespBodyLen"`
	RequestBodySanitizer  BodySanitizer `yaml:"-"`
	ResponseBodySanitizer BodySanitizer `yaml:"-"`

	// Skip is evaluated before the handler so skipped requests allocate no
	// capture buffers and never inspect request or response bodies.
	Skip func(ctx *gin.Context) bool `yaml:"-"`
}

func DefaultAccessLoggerConfig() AccessLoggerConfig {
	return AccessLoggerConfig{
		SkipPaths:      []string{},
		PrintHeaders:   []string{},
		MaxReqBodyLen:  _defaultPrintRequestLen,
		MaxRespBodyLen: _defaultPrintResponseLen,
	}
}

func mergeWithDefaultAccessLog(userConf AccessLoggerConfig) AccessLoggerConfig {
	defaultConf := DefaultAccessLoggerConfig()
	if userConf.SkipPaths == nil {
		userConf.SkipPaths = defaultConf.SkipPaths
	}
	if userConf.PrintHeaders == nil {
		userConf.PrintHeaders = defaultConf.PrintHeaders
	}
	if userConf.MaxReqBodyLen == 0 {
		userConf.MaxReqBodyLen = defaultConf.MaxReqBodyLen
	}
	if userConf.MaxRespBodyLen == 0 {
		userConf.MaxRespBodyLen = defaultConf.MaxRespBodyLen
	}
	return userConf
}

func AccessLog(conf AccessLoggerConfig) gin.HandlerFunc {
	skipPaths := make(map[string]struct{}, len(conf.SkipPaths))
	for _, path := range conf.SkipPaths {
		skipPaths[path] = struct{}{}
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Set(zlog.ContextKeyUri, path)
		_ = zlog.GetRequestID(c)

		if _, ok := skipPaths[path]; ok {
			c.Next()
			return
		}
		if conf.Skip != nil && conf.Skip(c) {
			c.Next()
			return
		}

		var requestCapture *boundedCapture
		var requestLog string
		if conf.MaxReqBodyLen > 0 && conf.RequestBodySanitizer != nil {
			if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodDelete {
				query := []byte(c.Request.URL.Query().Encode())
				capture := newBoundedCapture(conf.MaxReqBodyLen)
				_, _ = capture.Write(query)
				requestLog = sanitizeCapturedBody(
					conf.RequestBodySanitizer,
					"application/x-www-form-urlencoded",
					capture,
				)
			} else if c.Request.Body != nil {
				c.Request.Body, requestCapture = captureRequestBody(c.Request.Body, conf.MaxReqBodyLen)
			}
		}

		var responseCapture *boundedCapture
		if conf.MaxRespBodyLen > 0 && conf.ResponseBodySanitizer != nil {
			responseCapture = newBoundedCapture(conf.MaxRespBodyLen)
			c.Writer = &customRespWriter{
				ResponseWriter: c.Writer,
				body:           responseCapture,
			}
		}

		c.Next()

		if requestCapture != nil {
			requestLog = sanitizeCapturedBody(
				conf.RequestBodySanitizer,
				c.ContentType(),
				requestCapture,
			)
		}

		fields := []zlog.Field{
			zlog.String("method", c.Request.Method),
			zlog.String("uri", path),
			zlog.Int("status", c.Writer.Status()),
			zlog.String("clientIp", c.ClientIP()),
			zlog.Int("bodySize", c.Writer.Size()),
		}
		if requestLog != "" {
			fields = append(fields, zlog.String("requestBody", requestLog))
		}
		if headers := getHeader(c, conf.PrintHeaders); headers != "" {
			fields = append(fields, zlog.String("requestHeader", headers))
		}
		if responseLog := sanitizedResponseBody(c, conf, responseCapture); responseLog != "" {
			fields = append(fields, zlog.String("responseBody", responseLog))
		}

		fields = append(fields, AppendCostTime(start, time.Now())...)
		fields = append(fields, zlog.GetCustomerFields(c)...)
		zlog.AccessInfo(c, fields...)
	}
}

func sanitizeCapturedBody(sanitizer BodySanitizer, contentType string, capture *boundedCapture) string {
	if sanitizer == nil || capture == nil {
		return ""
	}
	value := sanitizer(contentType, capture.Bytes())
	if value != "" && capture.Truncated() {
		value += "...[truncated]"
	}
	return value
}

func sanitizedResponseBody(
	c *gin.Context,
	conf AccessLoggerConfig,
	capture *boundedCapture,
) string {
	if capture == nil || conf.ResponseBodySanitizer == nil {
		return ""
	}
	contentType := c.Writer.Header().Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType == "text/event-stream" {
		return ""
	}
	return sanitizeCapturedBody(conf.ResponseBodySanitizer, mediaType, capture)
}

var sensitiveRequestHeaders = map[string]struct{}{
	"Authorization":       {},
	"Cookie":              {},
	"Proxy-Authorization": {},
	"X-Api-Key":           {},
}

func getHeader(ctx *gin.Context, headers []string) string {
	values := make([]string, 0, len(headers))
	seen := make(map[string]struct{}, len(headers))
	for _, rawName := range headers {
		name := textproto.CanonicalMIMEHeaderKey(rawName)
		if _, sensitive := sensitiveRequestHeaders[name]; sensitive {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		if headerValues := ctx.Request.Header.Values(name); len(headerValues) > 0 {
			values = append(values, fmt.Sprintf("%s=%v", name, headerValues))
		}
	}
	return strings.Join(values, "&")
}

func RegistryAccessLog(engine *gin.Engine, conf ...AccessLoggerConfig) {
	logConf := DefaultAccessLoggerConfig()
	if len(conf) > 0 {
		logConf = mergeWithDefaultAccessLog(conf[0])
	}
	engine.Use(AccessLog(logConf))
}

func AppendCostTime(begin, end time.Time) []zlog.Field {
	return []zlog.Field{
		zlog.String("startTime", zlog.GetFormatRequestTime(begin)),
		zlog.String("endTime", zlog.GetFormatRequestTime(end)),
		zlog.String("cost", fmt.Sprintf("%vms", zlog.GetRequestCost(begin, end))),
	}
}
