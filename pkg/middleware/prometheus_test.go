package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMetricsUseRouteTemplateAndStatusClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics, err := NewMetrics(MetricsConfig{AppName: "test"})
	require.NoError(t, err)

	engine := gin.New()
	RegisterMetrics(engine, metrics)
	engine.GET("/users/:id", func(ctx *gin.Context) {
		ctx.Status(http.StatusCreated)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users/user-123", nil))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := response.Body.String()
	require.Contains(t, body, `route="/users/:id"`)
	require.Contains(t, body, `status_class="2xx"`)
	require.NotContains(t, body, `route="/users/user-123"`)
}

func TestMetricsHandlerAdaptsOwnedRegistryToHTTP(t *testing.T) {
	metrics, err := NewMetrics(MetricsConfig{AppName: "test"})
	require.NoError(t, err)
	response := httptest.NewRecorder()

	metrics.Handler().ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "go_gc_duration_seconds")
}
