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
)

var (
	globalLogger *zap.SugaredLogger
)

/*---------------sugar Logger-------------------*/

func GetGlobalLogger() *zap.SugaredLogger {
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	if globalLogger != nil {
		return globalLogger
	}
	globalLogger = newLoggerWithSkipLocked(1).Sugar()
	return globalLogger
}

func sugaredLogger(ctx context.Context) *zap.SugaredLogger {
	if ctx == nil {
		return NewLoggerWithSkip(1).Sugar()
	}
	return LoggerWithContext(NewLoggerWithSkip(1), ctx).Sugar()
}

func Debugf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Debugf(format, args...)
}

func Info(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Info(args...)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Infof(format, args...)
}

func Warn(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Warn(args...)
}

func Warnf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Warnf(format, args...)
}

func Error(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Error(args...)
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Errorf(format, args...)
}

func Panic(ctx context.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Panic(args...)
}

func Panicf(ctx context.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Panicf(format, args...)
}
