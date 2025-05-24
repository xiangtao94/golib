// Package algo -----------------------------
// @file      : request_id.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/5/24 18:14
// Description:
// -------------------------------------------
package zlog

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const (
	ContextKeyRequestID = "request_id"
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

var generator = newRand(time.Now().UnixNano())

func genRequestID() string {
	// 生成 uint64的随机数, 并转换成16进制表示方式
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%016x:0", uint64(generator.Int63())))
	return buffer.String()
}

type LockedSource struct {
	mut sync.Mutex
	src rand.Source
}

func newRand(seed int64) *rand.Rand {
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
