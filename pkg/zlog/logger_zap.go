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
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	zapLoggerAddr = "_zap_addr"
)

var (
	// key 为skip
	zapLoggerCache = make(map[int]*zap.Logger)
	zapCacheLock   sync.Mutex
)

// 通用 Logger 工厂，根据 skip 构造 Logger 实例, 定制化skip实例
func NewLoggerWithSkip(skip int) *zap.Logger {
	// 检查缓存
	zapCacheLock.Lock()
	if logger, exists := zapLoggerCache[skip]; exists {
		zapCacheLock.Unlock()
		return logger
	}
	// 构造新的 Logger
	core := buildZapCore(false)
	logger := zap.New(core, zap.Fields(), zap.WithCaller(true), zap.Development(), zap.AddCallerSkip(skip))
	zapLoggerCache[skip] = logger
	zapCacheLock.Unlock()
	return logger
}

func zapLogger(ctx context.Context) *zap.Logger {
	m := NewLoggerWithSkip(1)
	if ctx == nil {
		return m
	}

	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		if cached, exists := ginCtx.Get(zapLoggerAddr); exists {
			if logger, valid := cached.(*zap.Logger); valid {
				return logger
			}
		}
		logger := LoggerWithContext(m, ginCtx)
		ginCtx.Set(zapLoggerAddr, logger)
		return logger
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
