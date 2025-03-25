// Package middleware -----------------------------
// @file      : sse.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/3/25 15:08
// -------------------------------------------
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func UploadEventStream(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	ctx.Writer.Header().Set(zlog.ContextKeyRequestID, zlog.GetRequestID(ctx))
	origin := ctx.Request.Header.Get("Origin")
	if origin != "" {
		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE,UPDATE")
		ctx.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, cache-control, X-CSRF-Token, Token,session")
		ctx.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		ctx.Header("Access-Control-Allow-Credentials", "true")
	}
	ctx.Next()
}
