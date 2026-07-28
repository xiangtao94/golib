package lifecycle

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagerStartsInOrderAndStopsInReverseOrder(t *testing.T) {
	var events []string
	manager, err := New(
		Hook{
			Name:    "database",
			OnStart: func(context.Context) error { events = append(events, "start database"); return nil },
			OnStop:  func(context.Context) error { events = append(events, "stop database"); return nil },
		},
		Hook{
			Name:    "http",
			OnStart: func(context.Context) error { events = append(events, "start http"); return nil },
			OnStop:  func(context.Context) error { events = append(events, "stop http"); return nil },
		},
	)
	require.NoError(t, err)

	require.NoError(t, manager.Start(context.Background()))
	require.NoError(t, manager.Stop(context.Background()))

	require.Equal(t, []string{
		"start database",
		"start http",
		"stop http",
		"stop database",
	}, events)
}

func TestManagerRollsBackStartedHooksWhenStartFails(t *testing.T) {
	startErr := errors.New("listen failed")
	var events []string
	manager, err := New(
		Hook{
			Name:    "database",
			OnStart: func(context.Context) error { events = append(events, "start database"); return nil },
			OnStop:  func(context.Context) error { events = append(events, "stop database"); return nil },
		},
		Hook{
			Name:    "http",
			OnStart: func(context.Context) error { return startErr },
		},
	)
	require.NoError(t, err)

	err = manager.Start(context.Background())

	require.ErrorIs(t, err, startErr)
	require.Equal(t, []string{"start database", "stop database"}, events)
	require.NoError(t, manager.Stop(context.Background()))
}

func TestManagerJoinsAllStopErrorsAndStopIsIdempotent(t *testing.T) {
	firstErr := errors.New("first stop failed")
	secondErr := errors.New("second stop failed")
	stopCalls := 0
	manager, err := New(
		Hook{Name: "first", OnStop: func(context.Context) error { stopCalls++; return firstErr }},
		Hook{Name: "second", OnStop: func(context.Context) error { stopCalls++; return secondErr }},
	)
	require.NoError(t, err)
	require.NoError(t, manager.Start(context.Background()))

	err = manager.Stop(context.Background())
	repeatedErr := manager.Stop(context.Background())

	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.Equal(t, err, repeatedErr)
	require.Equal(t, 2, stopCalls)
}

func TestNewRejectsUnnamedAndEmptyHooks(t *testing.T) {
	_, err := New(Hook{})
	require.Error(t, err)

	_, err = New(Hook{Name: "empty"})
	require.Error(t, err)
}
