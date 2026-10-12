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

const maxRequestIDLength = 128

type requestIDContextKey struct{}

// WithRequestID returns a context that carries requestID without coupling
// application code to an HTTP framework.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		panic("zlog: nil context")
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// EnsureRequestID returns a context carrying a stable request ID. Invalid IDs
// from context or candidates are replaced with a generated ID.
func EnsureRequestID(ctx context.Context, candidates ...string) (context.Context, string) {
	if ctx == nil {
		panic("zlog: nil context")
	}
	if requestID := GetRequestID(ctx); validRequestID(requestID) {
		return ctx, requestID
	}
	for _, candidate := range candidates {
		if validRequestID(candidate) {
			return WithRequestID(ctx, candidate), candidate
		}
	}
	requestID := genRequestID()
	return WithRequestID(ctx, requestID), requestID
}

// validRequestID accepts a bounded, single-token ASCII identifier suitable
// for HTTP headers and structured logs.
func validRequestID(value string) bool {
	if len(value) == 0 || len(value) > maxRequestIDLength {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' ||
			char == '-' || char == '_' || char == '.' || char == ':' {
			continue
		}
		return false
	}
	return true
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
