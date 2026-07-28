package middleware

import (
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	serviceerrors "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
)

type RateLimitKeyFunc func(*gin.Context) string

type RateLimiterConfig struct {
	Rate       rate.Limit
	Burst      int
	TTL        time.Duration
	MaxEntries int
	Key        RateLimitKeyFunc
	Renderer   *render.JSONRenderer
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu          sync.Mutex
	entries     map[string]*limiterEntry
	rate        rate.Limit
	burst       int
	ttl         time.Duration
	maxEntries  int
	lastCleanup time.Time
}

func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	if config.Rate <= 0 {
		config.Rate = 1
	}
	if config.Burst <= 0 {
		config.Burst = 1
	}
	if config.TTL <= 0 {
		config.TTL = 10 * time.Minute
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = 10_000
	}
	return &RateLimiter{
		entries:     make(map[string]*limiterEntry),
		rate:        config.Rate,
		burst:       config.Burst,
		ttl:         config.TTL,
		maxEntries:  config.MaxEntries,
		lastCleanup: time.Now(),
	}
}

func (limiter *RateLimiter) allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if now.Sub(limiter.lastCleanup) >= limiter.ttl/2 {
		limiter.removeExpired(now)
		limiter.lastCleanup = now
	}
	entry, exists := limiter.entries[key]
	if !exists {
		if len(limiter.entries) >= limiter.maxEntries {
			limiter.evictOldest()
		}
		entry = &limiterEntry{limiter: rate.NewLimiter(limiter.rate, limiter.burst)}
		limiter.entries[key] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

func (limiter *RateLimiter) removeExpired(now time.Time) {
	for key, entry := range limiter.entries {
		if now.Sub(entry.lastSeen) >= limiter.ttl {
			delete(limiter.entries, key)
		}
	}
}

func (limiter *RateLimiter) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range limiter.entries {
		if oldestKey == "" || entry.lastSeen.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.lastSeen
		}
	}
	if oldestKey != "" {
		delete(limiter.entries, oldestKey)
	}
}

// DirectClientIP keys by the TCP peer and does not trust forwarded headers.
func DirectClientIP(ctx *gin.Context) string {
	host, _, err := net.SplitHostPort(ctx.Request.RemoteAddr)
	if err == nil {
		return host
	}
	return ctx.Request.RemoteAddr
}

// TrustedProxyClientIP uses Gin's configured trusted-proxy policy.
func TrustedProxyClientIP(ctx *gin.Context) string {
	return ctx.ClientIP()
}

func RateLimitMiddleware(config RateLimiterConfig) gin.HandlerFunc {
	limiter := NewRateLimiter(config)
	keyFunc := config.Key
	if keyFunc == nil {
		keyFunc = DirectClientIP
	}
	renderer := config.Renderer
	if renderer == nil {
		renderer = render.NewJSONRenderer(nil)
	}

	return func(ctx *gin.Context) {
		if !limiter.allow(keyFunc(ctx), time.Now()) {
			ctx.Abort()
			renderer.Failure(ctx, serviceerrors.ErrResourceExhausted)
			return
		}
		ctx.Next()
	}
}
