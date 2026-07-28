package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	serviceerrors "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
)

// TimeoutMiddleware 超时控制中间件
func TimeoutMiddleware(timeout time.Duration, renderer *render.JSONRenderer) gin.HandlerFunc {
	if renderer == nil {
		renderer = render.NewJSONRenderer(nil)
	}
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
			c.Abort()
			renderer.Failure(c, serviceerrors.ErrDeadlineExceeded)
		}
	}
}
