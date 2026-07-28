package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
