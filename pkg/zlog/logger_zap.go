// Package algo -----------------------------
// @file      : logger_zap.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/5/24 17:58
// Description:
// -------------------------------------------
package zlog

import (
	"context"

	"go.uber.org/zap"
)

var (
	// key 为skip
	zapLoggerCache = make(map[int]*zap.Logger)
)

// 通用 Logger 工厂，根据 skip 构造 Logger 实例, 定制化skip实例
func NewLoggerWithSkip(skip int) *zap.Logger {
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	return newLoggerWithSkipLocked(skip)
}

func newLoggerWithSkipLocked(skip int) *zap.Logger {
	if logger, exists := zapLoggerCache[skip]; exists {
		return logger
	}
	core := buildZapCore(false)
	logger := zap.New(core, zap.Fields(), zap.WithCaller(true), zap.Development(), zap.AddCallerSkip(skip))
	zapLoggerCache[skip] = logger
	return logger
}

func zapLogger(ctx context.Context) *zap.Logger {
	m := NewLoggerWithSkip(1)
	if ctx == nil {
		return m
	}
	return LoggerWithContext(m, ctx)
}

func DebugLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Debug(msg, fields...)
}

func InfoLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Info(msg, fields...)
}

func WarnLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Warn(msg, fields...)
}

func ErrorLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Error(msg, fields...)
}

func PanicLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Panic(msg, fields...)
}

func FatalLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Fatal(msg, fields...)
}
