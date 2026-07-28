package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TimeoutMiddleware 超时控制中间件
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		// Gin's context and response writer must stay on the request goroutine.
		// Handlers are responsible for observing ctx.Done and returning. If a
		// cooperative handler exits without writing, provide a consistent 504.
		if ctx.Err() == context.DeadlineExceeded &&
			!c.Writer.Written() &&
			c.Writer.Status() == http.StatusOK {
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{
				"code":    http.StatusGatewayTimeout,
				"message": "请求超时",
			})
		}
	}
}
