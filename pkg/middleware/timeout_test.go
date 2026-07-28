package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTimeoutMiddlewareDoesNotRunGinChainConcurrently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(TimeoutMiddleware(5*time.Millisecond, nil))
	engine.GET("/slow", func(c *gin.Context) {
		time.Sleep(30 * time.Millisecond)
		c.Status(http.StatusNoContent)
	})

	start := time.Now()
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/slow", nil))

	require.GreaterOrEqual(t, time.Since(start), 25*time.Millisecond)
	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestTimeoutMiddlewareReturnsGatewayTimeoutWhenHandlerHonorsCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(TimeoutMiddleware(5*time.Millisecond, nil))
	engine.GET("/cooperative", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/cooperative", nil))

	require.Equal(t, http.StatusGatewayTimeout, response.Code)
	require.Contains(t, response.Body.String(), `"code":"DEADLINE_EXCEEDED"`)
	require.Contains(t, response.Body.String(), `"reason":"TIMEOUT"`)
	require.Contains(t, response.Body.String(), `"request_id":`)
}
