package zlog

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestIDFromStandardContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")

	require.Equal(t, "req-123", GetRequestID(ctx))
}

func TestEnsureRequestIDUsesIncomingCandidate(t *testing.T) {
	ctx, requestID := EnsureRequestID(context.Background(), "req-456")

	require.Equal(t, "req-456", requestID)
	require.Equal(t, "req-456", GetRequestID(ctx))
}

func TestEnsureRequestIDAcceptsMaximumLength(t *testing.T) {
	candidate := strings.Repeat("a", maxRequestIDLength)
	_, requestID := EnsureRequestID(context.Background(), candidate)
	require.Equal(t, candidate, requestID)
}

func TestEnsureRequestIDReplacesInvalidCandidates(t *testing.T) {
	for _, candidate := range []string{
		"contains space",
		"line\nbreak",
		"comma,separated",
		"非ASCII",
		strings.Repeat("a", maxRequestIDLength+1),
	} {
		t.Run(candidate, func(t *testing.T) {
			ctx, requestID := EnsureRequestID(context.Background(), candidate)
			require.NotEqual(t, candidate, requestID)
			require.True(t, validRequestID(requestID))
			require.Equal(t, requestID, GetRequestID(ctx))
		})
	}
}

func TestEnsureRequestIDReplacesInvalidContextValue(t *testing.T) {
	ctx := WithRequestID(context.Background(), "invalid value")
	ctx, requestID := EnsureRequestID(ctx, "valid-id:1")

	require.Equal(t, "valid-id:1", requestID)
	require.Equal(t, requestID, GetRequestID(ctx))
}

func TestEnsureRequestIDIsStableForStandardContext(t *testing.T) {
	ctx, requestID := EnsureRequestID(context.Background())

	require.NotEmpty(t, requestID)
	require.Equal(t, requestID, GetRequestID(ctx))
	require.Equal(t, requestID, GetRequestID(ctx))
}

func TestLoggerWithContextIncludesAdditionalFields(t *testing.T) {
	core, observed := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	ctx := WithFields(
		WithRequestID(context.Background(), "req-123"),
		String("trace_id", "trace-123"),
		String("span_id", "span-123"),
	)

	LoggerWithContext(logger, ctx).Info("message")

	require.Equal(t, map[string]any{
		"requestId": "req-123",
		"trace_id":  "trace-123",
		"span_id":   "span-123",
	}, observed.All()[0].ContextMap())
}

func TestLoggerWithContextReusesBaseWhenContextHasNoLogFields(t *testing.T) {
	logger := zap.NewNop()

	require.Same(t, logger, LoggerWithContext(logger, context.Background()))
}
