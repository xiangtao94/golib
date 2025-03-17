package zlog

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// util key
const (
	ContextKeyRequestID = "request_id"
	ContextKeyNoLog     = "_no_log"
	ContextKeyUri       = "_uri"
	zapLoggerAddr       = "_zap_addr"
	sugaredLoggerAddr   = "_sugared_addr"
	customerFieldKey    = "__customerFields"
)

func GetRequestID(ctx *gin.Context) string {
	if ctx == nil {
		return genRequestID()
	}

	// 从ctx中获取
	if r := ctx.GetString(ContextKeyRequestID); r != "" {
		return r
	}
	// 请求头是上层传下来的
	var requestID string
	if ctx.Request != nil && ctx.Request.Header != nil {
		requestID = ctx.Request.Header.Get(ContextKeyRequestID)
	}
	if len(requestID) > 0 {
		if strings.Contains(requestID, ":") {
			tt := strings.Split(requestID, ":")
			requestID = fmt.Sprintf("%s:%016x", tt[0], uint64(generator.Int63()))
		}
		return requestID
	}
	requestID = genRequestID()
	ctx.Set(ContextKeyRequestID, requestID)
	return requestID
}

var generator = NewRand(time.Now().UnixNano())

func genRequestID() string {
	// 生成 uint64的随机数, 并转换成16进制表示方式
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%016x:0", uint64(generator.Int63())))
	return buffer.String()
}

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

type LockedSource struct {
	mut sync.Mutex
	src rand.Source
}

// NewRand returns a rand.Rand that is threadsafe.
func NewRand(seed int64) *rand.Rand {
	return rand.New(&LockedSource{src: rand.NewSource(seed)})
}

func (r *LockedSource) Int63() (n int64) {
	r.mut.Lock()
	n = r.src.Int63()
	r.mut.Unlock()
	return
}

// Seed implements Seed() of Source
func (r *LockedSource) Seed(seed int64) {
	r.mut.Lock()
	r.src.Seed(seed)
	r.mut.Unlock()
}

func AppendCostTime(begin, end time.Time) []Field {
	return []Field{
		String("startTime", GetFormatRequestTime(begin)),
		String("endTime", GetFormatRequestTime(end)),
		String("cost", fmt.Sprintf("%v%s", GetRequestCost(begin, end), "ms")),
	}
}
