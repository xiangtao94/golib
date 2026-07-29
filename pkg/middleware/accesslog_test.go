package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/zlog"
)

func TestDefaultAccessLoggerConfigDoesNotCaptureBodies(t *testing.T) {
	conf := DefaultAccessLoggerConfig()

	require.Equal(t, -1, conf.MaxReqBodyLen)
	require.Equal(t, -1, conf.MaxRespBodyLen)
	require.Nil(t, conf.RequestBodySanitizer)
	require.Nil(t, conf.ResponseBodySanitizer)
}

func TestBoundedCaptureKeepsOnlyPrefix(t *testing.T) {
	capture := newBoundedCapture(4)

	n, err := capture.Write([]byte("abcdef"))

	require.NoError(t, err)
	require.Equal(t, 6, n)
	require.Equal(t, "abcd", capture.String())
	require.True(t, capture.Truncated())
}

func TestCaptureRequestBodyPreservesFullBodyForHandler(t *testing.T) {
	body, capture := captureRequestBody(
		io.NopCloser(strings.NewReader("abcdef")),
		3,
	)

	data, err := io.ReadAll(body)

	require.NoError(t, err)
	require.Equal(t, "abcdef", string(data))
	require.Equal(t, "abc", capture.String())
	require.True(t, capture.Truncated())
}

func TestGetHeaderNeverReturnsSensitiveHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	ctx.Request.Header.Set("Authorization", "Bearer secret")
	ctx.Request.Header.Set("Cookie", "session=secret")
	ctx.Request.Header.Set("X-Trace-ID", "trace-123")

	headers := getHeader(ctx, []string{"Authorization", "Cookie", "X-Trace-ID"})

	require.Equal(t, "X-Trace-Id=[trace-123]", headers)
}

func TestAccessLogSkipsBodySanitizationWhenLoggingIsDisabled(t *testing.T) {
	_, err := zlog.InitLog(zlog.LogConfig{
		Level:  "info",
		Stdout: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, zlog.CloseLogger()) })

	sanitized := false
	engine := gin.New()
	engine.Use(AccessLog(AccessLoggerConfig{
		MaxRespBodyLen: 16,
		ResponseBodySanitizer: func(_ string, body []byte) string {
			sanitized = true
			return string(body)
		},
	}))
	engine.GET("/", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "response")
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.False(t, sanitized)
}

func TestAccessLogSkipsRequestURIContextWhenLoggingIsDisabled(t *testing.T) {
	_, err := zlog.InitLog(zlog.LogConfig{
		Level:  "info",
		Stdout: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, zlog.CloseLogger()) })

	var requestURI string
	engine := gin.New()
	engine.Use(AccessLog(DefaultAccessLoggerConfig()))
	engine.GET("/", func(ctx *gin.Context) {
		requestURI = zlog.GetRequestURI(ctx.Request.Context())
		ctx.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.Empty(t, requestURI)
}
