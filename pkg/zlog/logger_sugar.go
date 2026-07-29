// Package algo -----------------------------
// @file      : logger_sugar.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/5/24 17:29
// Description: 是 zap.Logger 的封装，提供了类似 fmt.Printf 风格的日志接口， 性能比zap.logger会低
// -------------------------------------------
package zlog

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.SugaredLogger
)

/*---------------sugar Logger-------------------*/

func GetGlobalLogger() *zap.SugaredLogger {
	loggerLifecycleMu.RLock()
	logger := globalLogger
	loggerLifecycleMu.RUnlock()
	if logger != nil {
		return logger
	}
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	if globalLogger != nil {
		return globalLogger
	}
	globalLogger = newLoggerWithSkipLocked(1).Sugar()
	return globalLogger
}

func sugaredLoggerForLevel(
	ctx context.Context,
	level zapcore.Level,
) (*zap.SugaredLogger, bool) {
	logger := NewLoggerWithSkip(1)
	if !logger.Core().Enabled(level) {
		return nil, false
	}
	return LoggerWithContext(logger, ctx).Sugar(), true
}

func Debugf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	logger, enabled := sugaredLoggerForLevel(ctx, zap.DebugLevel)
	if enabled {
		logger.Debugf(format, args...)
	}
}

func Info(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	logger, enabled := sugaredLoggerForLevel(ctx, zap.InfoLevel)
	if enabled {
		logger.Info(args...)
	}
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	logger, enabled := sugaredLoggerForLevel(ctx, zap.InfoLevel)
	if enabled {
		logger.Infof(format, args...)
	}
}

func Warn(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	logger, enabled := sugaredLoggerForLevel(ctx, zap.WarnLevel)
	if enabled {
		logger.Warn(args...)
	}
}

func Warnf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	logger, enabled := sugaredLoggerForLevel(ctx, zap.WarnLevel)
	if enabled {
		logger.Warnf(format, args...)
	}
}

func Error(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	logger, enabled := sugaredLoggerForLevel(ctx, zap.ErrorLevel)
	if enabled {
		logger.Error(args...)
	}
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	logger, enabled := sugaredLoggerForLevel(ctx, zap.ErrorLevel)
	if enabled {
		logger.Errorf(format, args...)
	}
}

func Panic(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	LoggerWithContext(NewLoggerWithSkip(1), ctx).Sugar().Panic(args...)
}

func Panicf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	LoggerWithContext(NewLoggerWithSkip(1), ctx).Sugar().Panicf(format, args...)
}
