package zlog

import (
	"fmt"
	"github.com/xiangtao94/golib/pkg/env"
	"go.uber.org/zap"
	"time"

	"github.com/gin-gonic/gin"
)

// util key
const (
	ContextKeyNoLog  = "_no_log"
	ContextKeyUri    = "_uri"
	customerFieldKey = "__customerFields"
)

func GetRequestUri(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.GetString(ContextKeyUri)
}

// a new method for customer notice
func AddField(c *gin.Context, field ...Field) {
	customerFields := GetCustomerFields(c)
	if customerFields == nil {
		customerFields = field
	} else {
		customerFields = append(customerFields, field...)
	}

	c.Set(customerFieldKey, customerFields)
}

// 获得所有用户自定义的Field
func GetCustomerFields(c *gin.Context) (customerFields []Field) {
	if v, exist := c.Get(customerFieldKey); exist {
		customerFields, _ = v.([]Field)
	}
	return customerFields
}

func SetNoLogFlag(ctx *gin.Context) {
	ctx.Set(ContextKeyNoLog, true)
}

func SetLogFlag(ctx *gin.Context) {
	ctx.Set(ContextKeyNoLog, false)
}

func noLog(ctx *gin.Context) bool {
	if ctx == nil {
		return false
	}
	flag, ok := ctx.Get(ContextKeyNoLog)
	if ok && flag == true {
		return true
	}
	return false
}

func GetFormatRequestTime(time time.Time) string {
	return time.Format("2006-01-02 15:04:05.000")
}

func GetRequestCost(start, end time.Time) float64 {
	return float64(end.Sub(start).Nanoseconds()/1e4) / 100.0
}

func AppendCostTime(begin, end time.Time) []Field {
	return []Field{
		String("startTime", GetFormatRequestTime(begin)),
		String("endTime", GetFormatRequestTime(end)),
		String("cost", fmt.Sprintf("%v%s", GetRequestCost(begin, end), "ms")),
	}
}

// 返回带上下文信息的 zap.Logger
func LoggerWithContext(baseLogger *zap.Logger, ctx *gin.Context) *zap.Logger {
	if ctx == nil || baseLogger == nil {
		return baseLogger
	}
	return baseLogger.With(
		String("requestId", GetRequestID(ctx)),
		String("uri", GetRequestUri(ctx)),
		String("localIp", env.LocalIP),
	)
}
