package redis_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"github.com/xiangtao94/golib/pkg/redis"
)

func transactionClient(t *testing.T) *redis.Redis {
	t.Helper()
	server := miniredis.RunT(t)
	client, err := redis.InitRedisClient(redis.RedisConf{Addr: server.Addr(), AllowInsecureTransport: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestWatchWithRetryReevaluatesAfterConflict(t *testing.T) {
	client := transactionClient(t)
	ctx := context.Background()
	require.NoError(t, client.Set(ctx, "counter", 0, 0).Err())
	attempts := 0
	err := client.WatchWithRetry(ctx, func(tx *redis.Tx) error {
		attempts++
		value, err := tx.Get(ctx, "counter").Int()
		if err != nil {
			return err
		}
		if attempts == 1 {
			require.NoError(t, client.Set(ctx, "counter", 10, 0).Err())
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, "counter", value+1, 0)
			return nil
		})
		return err
	}, "counter")
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, "11", client.Get(ctx, "counter").Val())
}

func TestWatchWithRetryStopsOnCallbackError(t *testing.T) {
	client := transactionClient(t)
	expected := errors.New("business rejection")
	attempts := 0
	err := client.WatchWithRetry(context.Background(), func(tx *redis.Tx) error { attempts++; return expected }, "key")
	require.ErrorIs(t, err, expected)
	require.Equal(t, 1, attempts)
}

func TestWatchWithRetryHonorsCancellation(t *testing.T) {
	client := transactionClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	err := client.WatchWithRetry(ctx, func(tx *redis.Tx) error { attempts++; return nil }, "key")
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, attempts)
}

func TestWatchWithRetryCancelsDuringConflict(t *testing.T) {
	client := transactionClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempts := 0
	err := client.WatchWithRetry(ctx, func(tx *redis.Tx) error {
		attempts++
		cancel()
		return redis.ErrTxConflict
	}, "key")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, attempts)
}

func TestWatchWithRetryHasBoundedAttempts(t *testing.T) {
	client := transactionClient(t)
	ctx := context.Background()
	attempts := 0
	err := client.WatchWithRetry(ctx, func(tx *redis.Tx) error {
		attempts++
		require.NoError(t, client.Incr(ctx, "key").Err())
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, "key", 0, 0); return nil })
		return err
	}, "key")
	require.ErrorIs(t, err, redis.ErrTxConflict)
	require.Equal(t, 32, attempts)
	require.Equal(t, "32", client.Get(ctx, "key").Val())
}

func TestWatchWithRetryDoesNotRetryConnectionFailure(t *testing.T) {
	client := transactionClient(t)
	require.NoError(t, client.Close())
	called := false
	err := client.WatchWithRetry(context.Background(), func(tx *redis.Tx) error { called = true; return nil }, "key")
	require.Error(t, err)
	require.NotErrorIs(t, err, redis.ErrTxConflict)
	require.False(t, called)
}
