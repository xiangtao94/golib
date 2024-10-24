package rmq

import (
	"github.com/tiant-go/golib/pkg/env"
	"github.com/tiant-go/golib/pkg/zlog"
	"os"
	"sync"

	"github.com/apache/rocketmq-client-go/v2/rlog"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap/zapcore"
)

var logger *zlog.Logger

func initLogger() {
	if logger != nil {
		return
	}

	logger = zlog.ZapLogger.WithOptions(zlog.AddCallerSkip(-1))

	rlog.SetLogger(&rlogger{})
}

func fields(fields ...zlog.Field) []zlog.Field {
	commonFields := []zlog.Field{
		zlog.String("module", env.GetAppName()),
		zlog.String("localIp", env.LocalIP),
		zlog.String("prot", "mq"),
	}

	commonFields = append(commonFields, fields...)

	return commonFields
}

// ----------------------- sdk debug log -----------------------
type rlogger struct {
	initOnce sync.Once
	verbose  bool
}

func (r *rlogger) Level(level string) {}

func (r *rlogger) isVerbose() bool {
	r.initOnce.Do(func() {
		if os.Getenv("RMQ_SDK_VERBOSE") != "" {
			r.verbose = true
		} else {
			r.verbose = false
		}
	})
	return r.verbose
}

func (r *rlogger) getFields(fields map[string]interface{}) []zapcore.Field {
	var f = []zlog.Field{}
	for k, v := range fields {
		f = append(f, zlog.Reflect(k, v))
	}
	return f
}

func (r *rlogger) Debug(msg string, fields map[string]interface{}) {
	if r.isVerbose() {
		zlog.DebugLogger(nil, msg, r.getFields(fields)...)
	}
}

func (r *rlogger) Info(msg string, fields map[string]interface{}) {
	if r.isVerbose() {
		zlog.InfoLogger(nil, msg, r.getFields(fields)...)
	}
}

func (r *rlogger) Warning(msg string, fields map[string]interface{}) {
	if r.isVerbose() {
		zlog.WarnLogger(nil, msg, r.getFields(fields)...)
	}
}

func (r *rlogger) Error(msg string, fields map[string]interface{}) {
	zlog.ErrorLogger(nil, msg, r.getFields(fields)...)
}

func (r *rlogger) Fatal(msg string, fields map[string]interface{}) {
	zlog.FatalLogger(nil, msg, r.getFields(fields)...)
}

func (r *rlogger) OutputPath(path string) (err error) {
	return nil
}

func contextFields(ctx *gin.Context, fields ...zlog.Field) []zlog.Field {
	commonFields := []zlog.Field{
		zlog.String("requestId", zlog.GetRequestID(ctx)),
		zlog.String("module", env.GetAppName()),
		zlog.String("localIp", env.LocalIP),
		zlog.String("uri", zlog.GetRequestUri(ctx)),
		zlog.String("prot", "mq"),
	}
	commonFields = append(commonFields, fields...)
	return commonFields
}
