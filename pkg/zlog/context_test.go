package zlog

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestIDFromStandardContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")

	require.Equal(t, "req-123", GetRequestID(ctx))
}

func TestRequestIDRemainsCompatibleWithGinContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	ctx.Request.Header.Set(HeaderRequestID, "req-456")

	require.Equal(t, "req-456", GetRequestID(ctx))
}

func TestEnsureRequestIDIsStableForStandardContext(t *testing.T) {
	ctx, requestID := EnsureRequestID(context.Background())

	require.NotEmpty(t, requestID)
	require.Equal(t, requestID, GetRequestID(ctx))
	require.Equal(t, requestID, GetRequestID(ctx))
}
