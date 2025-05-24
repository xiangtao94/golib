// Package algo -----------------------------
// @file      : logger_access.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/5/24 18:04
// Description:
// -------------------------------------------
package zlog

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	zapAccessLoggerAddr = "_access_zap_addr"
)

var (
	accessLogger *zap.Logger
)

// GetAccessLogger 获取 Access Logger 实例
func GetAccessLogger() *zap.Logger {
	if accessLogger == nil {
		core := buildZapCore(true)
		accessLogger = zap.New(core, zap.Fields(), zap.WithCaller(true), zap.Development(), zap.AddCallerSkip(1))
	}
	return accessLogger
}

func zapAccessLogger(ctx *gin.Context) *zap.Logger {
	m := GetAccessLogger()
	if ctx == nil {
		return m
	}
	// 上下文存在就返回
	if t, exist := ctx.Get(zapAccessLoggerAddr); exist {
		if l, ok := t.(*zap.Logger); ok {
			return l
		}
	}
	l := LoggerWithContext(m, ctx)
	ctx.Set(zapAccessLoggerAddr, l)
	return l
}

func AccessInfo(ctx *gin.Context, fields ...zap.Field) {
	zapAccessLogger(ctx).Info("accesslog", fields...)
}
