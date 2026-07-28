// Package algo -----------------------------
// @file      : logger_access.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/5/24 18:04
// Description:
// -------------------------------------------
package zlog

import (
	"context"

	"go.uber.org/zap"
)

var (
	accessLogger *zap.Logger
)

// GetAccessLogger 获取 Access Logger 实例
func GetAccessLogger() *zap.Logger {
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	if accessLogger == nil {
		core := buildZapCore(true)
		accessLogger = zap.New(core, zap.Fields(), zap.WithCaller(true), zap.Development(), zap.AddCallerSkip(1))
	}
	return accessLogger
}

func zapAccessLogger(ctx context.Context) *zap.Logger {
	m := GetAccessLogger()
	if ctx == nil {
		return m
	}
	l := LoggerWithContext(m, ctx)
	l = l.With(
		String("uri", GetRequestURI(ctx)),
	)
	return l
}

func AccessInfo(ctx context.Context, fields ...zap.Field) {
	zapAccessLogger(ctx).Info("accesslog", fields...)
}
