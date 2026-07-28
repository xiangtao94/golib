package middleware

import (
	"testing"
	"time"

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
