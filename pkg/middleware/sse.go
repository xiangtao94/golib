package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/xiangtao94/golib/pkg/zlog"
)

// UploadEventStream configures SSE transport headers only. Cross-origin
// policy must be applied separately through NewCORS.
func UploadEventStream(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Header(zlog.HeaderRequestID, zlog.GetRequestID(ctx))
	ctx.Next()
}
