package zlog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestEnsureRequestIDIsStableForStandardContext(t *testing.T) {
	ctx, requestID := EnsureRequestID(context.Background())

	require.NotEmpty(t, requestID)
	require.Equal(t, requestID, GetRequestID(ctx))
	require.Equal(t, requestID, GetRequestID(ctx))
}
