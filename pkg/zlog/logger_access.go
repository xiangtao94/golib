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
	loggerLifecycleMu.RLock()
	logger := accessLogger
	loggerLifecycleMu.RUnlock()
	if logger != nil {
		return logger
	}
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	if accessLogger == nil {
		core := buildZapCore(true)
		accessLogger = zap.New(core, zap.Fields(), zap.WithCaller(true), zap.Development(), zap.AddCallerSkip(1))
	}
	return accessLogger
}

func zapAccessLogger(ctx context.Context) (*zap.Logger, bool) {
	if !AccessEnabled(ctx) {
		return nil, false
	}
	logger := GetAccessLogger()
	logger = LoggerWithContext(logger, ctx)
	if uri := GetRequestURI(ctx); uri != "" {
		logger = logger.With(String("uri", uri))
	}
	return logger, true
}

// AccessEnabled reports whether access-log work should be performed for ctx.
func AccessEnabled(ctx context.Context) bool {
	if noLog(ctx) {
		return false
	}
	return GetAccessLogger().Core().Enabled(zap.InfoLevel)
}

func AccessInfo(ctx context.Context, fields ...zap.Field) {
	logger, enabled := zapAccessLogger(ctx)
	if enabled {
		logger.Info("accesslog", fields...)
	}
}
