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
	"go.uber.org/zap/zapcore"
)

var (
	// key 为skip
	zapLoggerCache = make(map[int]*zap.Logger)
)

const maxCachedCallerSkip = 16

// 通用 Logger 工厂，根据 skip 构造 Logger 实例, 定制化skip实例
func NewLoggerWithSkip(skip int) *zap.Logger {
	if skip >= 0 && skip <= maxCachedCallerSkip {
		loggerLifecycleMu.RLock()
		logger := zapLoggerCache[skip]
		loggerLifecycleMu.RUnlock()
		if logger != nil {
			return logger
		}
	}
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	return newLoggerWithSkipLocked(skip)
}

func newLoggerWithSkipLocked(skip int) *zap.Logger {
	if skip >= 0 && skip <= maxCachedCallerSkip {
		if logger, exists := zapLoggerCache[skip]; exists {
			return logger
		}
	}
	core := buildZapCore(false)
	logger := zap.New(core, zap.Fields(), zap.WithCaller(true), zap.Development(), zap.AddCallerSkip(skip))
	if skip >= 0 && skip <= maxCachedCallerSkip {
		zapLoggerCache[skip] = logger
	}
	return logger
}

func zapLoggerForLevel(
	ctx context.Context,
	level zapcore.Level,
) (*zap.Logger, bool) {
	logger := NewLoggerWithSkip(1)
	if !logger.Core().Enabled(level) {
		return nil, false
	}
	return LoggerWithContext(logger, ctx), true
}

func DebugLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	logger, enabled := zapLoggerForLevel(ctx, zap.DebugLevel)
	if enabled {
		logger.Debug(msg, fields...)
	}
}

func InfoLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	logger, enabled := zapLoggerForLevel(ctx, zap.InfoLevel)
	if enabled {
		logger.Info(msg, fields...)
	}
}

func WarnLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	logger, enabled := zapLoggerForLevel(ctx, zap.WarnLevel)
	if enabled {
		logger.Warn(msg, fields...)
	}
}

func ErrorLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	logger, enabled := zapLoggerForLevel(ctx, zap.ErrorLevel)
	if enabled {
		logger.Error(msg, fields...)
	}
}

func PanicLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	LoggerWithContext(NewLoggerWithSkip(1), ctx).Panic(msg, fields...)
}

func FatalLogger(ctx context.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	logger, enabled := zapLoggerForLevel(ctx, zap.FatalLevel)
	if enabled {
		logger.Fatal(msg, fields...)
	}
}
