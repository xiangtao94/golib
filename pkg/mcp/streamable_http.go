// Package mcp -----------------------------
// @file      : server.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/6/16 01:57
// -------------------------------------------
package mcp

import (
	"fmt"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/server"
	"github.com/xiangtao94/golib/pkg/zlog"
)

// Register 注册MCP路由到Gin引擎
func (h *Handler) Register(r *gin.Engine) {
	// 创建SSE服务器选项
	shOpts := slices.Clone(h.StreamableHTTPOpts)

	shOpts = append(shOpts, server.WithEndpointPath(h.BasePath))
	shOpts = append(shOpts, server.WithLogger(newLogger()))
	sseServer := server.NewStreamableHTTPServer(h.server, shOpts...)

	r.POST(h.BasePath, func(c *gin.Context) {
		sseServer.ServeHTTP(c.Writer, c.Request)
	})
	r.GET(h.BasePath, func(c *gin.Context) {
		sseServer.ServeHTTP(c.Writer, c.Request)
	})
	r.DELETE(h.BasePath, func(c *gin.Context) {
		sseServer.ServeHTTP(c.Writer, c.Request)
	})
}

type mcpLogger struct {
	logger *zlog.Logger
}

func (l *mcpLogger) Infof(format string, v ...any) {
	l.logger.Info(fmt.Sprintf(format, v...))

}

func (l mcpLogger) Errorf(format string, v ...any) {
	l.logger.Error(fmt.Sprintf(format, v...))
}

func newLogger() *mcpLogger {
	return &mcpLogger{
		logger: zlog.NewLoggerWithSkip(3),
	}
}
