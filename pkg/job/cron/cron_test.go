package cron

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

type immediateSchedule struct{}

func (immediateSchedule) Next(now time.Time) time.Time {
	return now.Add(time.Millisecond)
}

func TestCronUsesCallerContextAndStops(t *testing.T) {
	scheduler := New(time.Local)
	require.NoError(t, scheduler.AddJob("* * * * * *", FuncJob(func(ctx context.Context) error {
		require.NotNil(t, ctx)
		return nil
	})))

	scheduler.Start(context.Background())
	scheduler.Start(context.Background())

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, scheduler.Stop(stopCtx))
	require.NoError(t, scheduler.Stop(stopCtx))
}

func TestCronRejectsNilLifecycleContexts(t *testing.T) {
	scheduler := New(time.Local)

	require.PanicsWithValue(t, "cron: nil context", func() {
		//lint:ignore SA1012 This test verifies that nil contexts are rejected.
		scheduler.Start(nil)
	})
	require.PanicsWithValue(t, "cron: nil context", func() {
		//lint:ignore SA1012 This test verifies that nil contexts are rejected.
		_ = scheduler.Stop(nil)
	})
}

func TestCronWaitsForInflightJobs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		scheduler := New(time.Local)
		started := make(chan struct{})
		release := make(chan struct{})
		scheduler.Schedule("immediate", immediateSchedule{}, FuncJob(func(context.Context) error {
			select {
			case <-started:
			default:
				close(started)
			}
			<-release
			return nil
		}))
		scheduler.Start(t.Context())
		<-started

		stopCtx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		stopped := make(chan error, 1)
		go func() { stopped <- scheduler.Stop(stopCtx) }()
		synctest.Wait()

		select {
		case <-stopped:
			t.Fatal("Stop returned before the in-flight job completed")
		default:
		}
		close(release)
		require.NoError(t, <-stopped)
	})
}

func TestCronDoesNotOverlapTheSameEntry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		scheduler := New(time.Local)
		var running atomic.Int32
		var maximum atomic.Int32
		scheduler.Schedule("immediate", immediateSchedule{}, FuncJob(func(ctx context.Context) error {
			current := running.Add(1)
			defer running.Add(-1)
			for {
				observed := maximum.Load()
				if current <= observed || maximum.CompareAndSwap(observed, current) {
					break
				}
			}
			<-ctx.Done()
			return nil
		}))

		scheduler.Start(t.Context())
		time.Sleep(20 * time.Millisecond)
		synctest.Wait()

		if got := maximum.Load(); got != 1 {
			t.Fatalf("maximum concurrent runs = %d, want 1", got)
		}
		stopCtx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		require.NoError(t, scheduler.Stop(stopCtx))
	})
}
