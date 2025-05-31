package middleware

import (
	"golang.org/x/time/rate"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 限流器结构
type RateLimiter struct {
	ips   map[string]*rate.Limiter
	mu    *sync.RWMutex
	rate  rate.Limit
	burst int
	ttl   time.Duration
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(r rate.Limit, b int, ttl time.Duration) *RateLimiter {
	return &RateLimiter{
		ips:   make(map[string]*rate.Limiter),
		mu:    &sync.RWMutex{},
		rate:  r,
		burst: b,
		ttl:   ttl,
	}
}

// getLimiter 获取指定IP的限流器
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.ips[ip] = limiter
	}

	return limiter
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(r rate.Limit, b int, ttl time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(r, b, ttl)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.getLimiter(ip).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    http.StatusTooManyRequests,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
