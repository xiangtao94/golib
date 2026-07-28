package cycle

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCyclePropagatesCancellationAndStops(t *testing.T) {
	scheduler := New()
	started := make(chan struct{})
	stopped := make(chan struct{})
	scheduler.AddFunc(time.Hour, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	})

	scheduler.Start(context.Background())
	<-started

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, scheduler.Stop(stopCtx))
	<-stopped
	require.NoError(t, scheduler.Stop(stopCtx))
}

func TestCycleRejectsNilLifecycleContexts(t *testing.T) {
	scheduler := New()

	require.PanicsWithValue(t, "cycle: nil context", func() {
		//lint:ignore SA1012 This test verifies that nil contexts are rejected.
		scheduler.Start(nil)
	})
	require.PanicsWithValue(t, "cycle: nil context", func() {
		//lint:ignore SA1012 This test verifies that nil contexts are rejected.
		_ = scheduler.Stop(nil)
	})
}

func TestCycleRetriesPanicsAsErrors(t *testing.T) {
	scheduler := New()
	var attempts atomic.Int32
	entry := &Entry{
		Job: FuncJob(func(context.Context) error {
			if attempts.Add(1) == 1 {
				panic("boom")
			}
			return nil
		}),
		MaxRetry:      1,
		RetryInterval: time.Millisecond,
	}

	scheduler.runWithRetry(context.Background(), entry)

	require.Equal(t, int32(2), attempts.Load())
}

func TestCycleStopHonorsDeadline(t *testing.T) {
	scheduler := New()
	started := make(chan struct{})
	scheduler.AddFunc(time.Hour, func(context.Context) error {
		close(started)
		time.Sleep(50 * time.Millisecond)
		return errors.New("late")
	})
	scheduler.Start(context.Background())
	<-started

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	require.ErrorIs(t, scheduler.Stop(stopCtx), context.DeadlineExceeded)
}
