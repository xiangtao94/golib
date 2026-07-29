package cron

import (
	"context"
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
	scheduler := New()
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
	scheduler := New()

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
		scheduler := New()
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
