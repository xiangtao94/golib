package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/zlog"
)

func TestRequestIDMovesIncomingHeaderToStandardContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	var requestID string
	engine.GET("/", func(ctx *gin.Context) {
		requestID = zlog.GetRequestID(ctx.Request.Context())
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(zlog.HeaderRequestID, "request-from-client")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "request-from-client", requestID)
	require.Equal(t, "request-from-client", response.Header().Get(zlog.HeaderRequestID))
}

func TestRequestIDReplacesInvalidIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	var requestID string
	engine.GET("/", func(ctx *gin.Context) {
		requestID = zlog.GetRequestID(ctx.Request.Context())
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(zlog.HeaderRequestID, "bad id")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.NotEmpty(t, requestID)
	require.NotEqual(t, "bad id", requestID)
	require.Equal(t, requestID, response.Header().Get(zlog.HeaderRequestID))
}
