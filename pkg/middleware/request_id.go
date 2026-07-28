package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/xiangtao94/golib/pkg/zlog"
)

func ensureRequestID(ctx *gin.Context) string {
	requestContext, requestID := zlog.EnsureRequestID(
		ctx.Request.Context(),
		ctx.GetHeader(zlog.HeaderRequestID),
	)
	ctx.Request = ctx.Request.WithContext(requestContext)
	ctx.Header(zlog.HeaderRequestID, requestID)
	return requestID
}

// RequestID moves the incoming request ID into the standard request context,
// or creates one when the header is absent.
func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ensureRequestID(ctx)
		ctx.Next()
	}
}
