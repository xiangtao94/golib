// Package algo -----------------------------
// @file      : request_id.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/5/24 18:14
// Description:
// -------------------------------------------
package zlog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const HeaderRequestID = "X-Request-ID"

type requestIDContextKey struct{}

// WithRequestID returns a context that carries requestID without coupling
// application code to an HTTP framework.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		panic("zlog: nil context")
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// EnsureRequestID returns a context carrying a stable request ID. The first
// non-empty candidate is used when the context does not already contain one.
func EnsureRequestID(ctx context.Context, candidates ...string) (context.Context, string) {
	if ctx == nil {
		panic("zlog: nil context")
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		return ctx, requestID
	}
	for _, candidate := range candidates {
		if candidate != "" {
			return WithRequestID(ctx, candidate), candidate
		}
	}
	requestID := genRequestID()
	return WithRequestID(ctx, requestID), requestID
}

func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if requestID, ok := ctx.Value(requestIDContextKey{}).(string); ok && requestID != "" {
		return requestID
	}
	return ""
}

func genRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("zlog: generate request ID: " + err.Error())
	}
	return hex.EncodeToString(value[:])
}
