package zlog

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type noLogContextKey struct{}
type requestURIContextKey struct{}
type fieldsContextKey struct{}

func WithFields(ctx context.Context, fields ...Field) context.Context {
	if ctx == nil {
		panic("zlog: nil context")
	}
	current, _ := ctx.Value(fieldsContextKey{}).([]Field)
	combined := make([]Field, 0, len(current)+len(fields))
	combined = append(combined, current...)
	combined = append(combined, fields...)
	return context.WithValue(ctx, fieldsContextKey{}, combined)
}

func GetRequestURI(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	uri, _ := ctx.Value(requestURIContextKey{}).(string)
	return uri
}

func WithRequestURI(ctx context.Context, uri string) context.Context {
	if ctx == nil {
		panic("zlog: nil context")
	}
	return context.WithValue(ctx, requestURIContextKey{}, uri)
}

func WithNoLog(ctx context.Context) context.Context {
	if ctx == nil {
		panic("zlog: nil context")
	}
	return context.WithValue(ctx, noLogContextKey{}, true)
}

func noLog(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	flag, _ := ctx.Value(noLogContextKey{}).(bool)
	return flag
}

func GetFormatRequestTime(time time.Time) string {
	return time.Format("2006-01-02 15:04:05.000")
}

func GetRequestCost(start, end time.Time) float64 {
	return float64(end.Sub(start).Nanoseconds()/1e4) / 100.0
}

// 返回带上下文信息的 zap.Logger
func LoggerWithContext(baseLogger *zap.Logger, ctx context.Context) *zap.Logger {
	if ctx == nil || baseLogger == nil {
		return baseLogger
	}
	fields := []Field{String("requestId", GetRequestID(ctx))}
	if contextual, ok := ctx.Value(fieldsContextKey{}).([]Field); ok {
		fields = append(fields, contextual...)
	}
	return baseLogger.With(fields...)
}
