package cron

import (
	"context"
	"testing"
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

func TestCronWaitsForInflightJobs(t *testing.T) {
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
	scheduler.Start(context.Background())
	<-started

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- scheduler.Stop(stopCtx) }()

	select {
	case <-stopped:
		t.Fatal("Stop returned before the in-flight job completed")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-stopped)
}
