// Package algo -----------------------------
// @file      : logger_sugar.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/5/24 17:29
// Description: 是 zap.Logger 的封装，提供了类似 fmt.Printf 风格的日志接口， 性能比zap.logger会低
// -------------------------------------------
package zlog

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	sugaredLoggerAddr = "_sugared_addr"
)

var (
	globalLogger *zap.SugaredLogger
)

/*---------------sugar Logger-------------------*/

func GetGlobalLogger() *zap.SugaredLogger {
	if globalLogger != nil {
		return globalLogger
	}
	// 初始化 globalLogger
	globalLogger = NewLoggerWithSkip(1).Sugar()
	return globalLogger
}

func sugaredLogger(ctx *gin.Context) *zap.SugaredLogger {
	if ctx == nil {
		return NewLoggerWithSkip(1).Sugar()
	}

	if t, exist := ctx.Get(sugaredLoggerAddr); exist {
		if s, ok := t.(*zap.SugaredLogger); ok {
			return s
		}
	}
	s := LoggerWithContext(NewLoggerWithSkip(1), ctx)
	ctx.Set(sugaredLoggerAddr, s)
	return s.Sugar()
}

func Debugf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Debugf(format, args...)
}

func Info(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Info(args...)
}

func Infof(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Infof(format, args...)
}

func Warn(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Warn(args...)
}

func Warnf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Warnf(format, args...)
}

func Error(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Error(args...)
}

func Errorf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Errorf(format, args...)
}

func Panic(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Panic(args...)
}

func Panicf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Panicf(format, args...)
}
