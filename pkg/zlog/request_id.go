// Package algo -----------------------------
// @file      : request_id.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/5/24 18:14
// Description:
// -------------------------------------------
package zlog

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ContextKeyRequestID = "request_id"
	HeaderRequestID     = "X-Request-ID"
)

type requestIDContextKey struct{}

// WithRequestID returns a context that carries requestID without coupling
// application code to an HTTP framework.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// EnsureRequestID returns a context carrying a stable request ID. Gin contexts
// are populated in place; immutable standard contexts are wrapped.
func EnsureRequestID(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if requestID, ok := ctx.Value(requestIDContextKey{}).(string); ok && requestID != "" {
		return ctx, requestID
	}
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		return ginCtx, GetRequestID(ginCtx)
	}
	requestID := genRequestID()
	return WithRequestID(ctx, requestID), requestID
}

func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return genRequestID()
	}

	if requestID, ok := ctx.Value(requestIDContextKey{}).(string); ok && requestID != "" {
		return requestID
	}

	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		if requestID := ginCtx.GetString(ContextKeyRequestID); requestID != "" {
			return requestID
		}

		var requestID string
		if ginCtx.Request != nil && ginCtx.Request.Header != nil {
			requestID = ginCtx.Request.Header.Get(HeaderRequestID)
			if requestID == "" {
				requestID = ginCtx.Request.Header.Get(ContextKeyRequestID)
			}
		}
		if requestID != "" {
			if strings.Contains(requestID, ":") {
				parts := strings.Split(requestID, ":")
				requestID = fmt.Sprintf("%s:%016x", parts[0], uint64(generator.Int63()))
			}
			ginCtx.Set(ContextKeyRequestID, requestID)
			return requestID
		}

		requestID = genRequestID()
		ginCtx.Set(ContextKeyRequestID, requestID)
		return requestID
	}

	return genRequestID()
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
