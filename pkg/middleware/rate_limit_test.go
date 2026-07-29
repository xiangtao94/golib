package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestRateLimiterExpiresAndBoundsEntries(t *testing.T) {
	limiter := NewRateLimiter(RateLimiterConfig{
		Rate:       rate.Inf,
		Burst:      1,
		TTL:        time.Second,
		MaxEntries: 2,
	})
	start := time.Now()

	require.True(t, limiter.allow("first", start))
	require.True(t, limiter.allow("second", start.Add(time.Millisecond)))
	require.True(t, limiter.allow("third", start.Add(2*time.Millisecond)))
	require.Len(t, limiter.entries, 2)
	require.NotContains(t, limiter.entries, "first")

	require.True(t, limiter.allow("fourth", start.Add(2*time.Second)))
	require.Len(t, limiter.entries, 1)
	require.Contains(t, limiter.entries, "fourth")
}

func TestRateLimiterRefreshesEntryBeforeConstantTimeEviction(t *testing.T) {
	limiter := NewRateLimiter(RateLimiterConfig{
		Rate:       rate.Inf,
		Burst:      1,
		TTL:        time.Hour,
		MaxEntries: 2,
	})
	start := time.Now()

	require.True(t, limiter.allow("first", start))
	require.True(t, limiter.allow("second", start.Add(time.Millisecond)))
	require.True(t, limiter.allow("first", start.Add(2*time.Millisecond)))
	require.True(t, limiter.allow("third", start.Add(3*time.Millisecond)))

	require.Contains(t, limiter.entries, "first")
	require.NotContains(t, limiter.entries, "second")
	require.Contains(t, limiter.entries, "third")
	require.Equal(t, 2, limiter.order.Len())
}

func TestRateLimitMiddlewareUsesTheSharedErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RateLimitMiddleware(RateLimiterConfig{
		Rate:  1,
		Burst: 1,
		Key:   func(*gin.Context) string { return "client" },
	}))
	engine.GET("/", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusNoContent, first.Code)

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.Contains(t, second.Body.String(), `"code":"RESOURCE_EXHAUSTED"`)
	require.Contains(t, second.Body.String(), `"reason":"RATE_LIMITED"`)
	require.Contains(t, second.Body.String(), `"request_id":`)
}
