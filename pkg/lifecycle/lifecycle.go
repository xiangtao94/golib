// Package lifecycle coordinates explicitly owned application resources.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrNilContext   = errors.New("lifecycle: nil context")
	ErrInvalidState = errors.New("lifecycle: invalid state")
)

type Hook struct {
	Name    string
	OnStart func(context.Context) error
	OnStop  func(context.Context) error
}

type state uint8

const (
	stateNew state = iota
	stateStarting
	stateStarted
	stateStopping
	stateStopped
)

type Manager struct {
	mu      sync.Mutex
	hooks   []Hook
	started []Hook
	state   state
	done    chan struct{}
	stopErr error
}

func New(hooks ...Hook) (*Manager, error) {
	for index, hook := range hooks {
		if strings.TrimSpace(hook.Name) == "" {
			return nil, fmt.Errorf("lifecycle: hook %d has no name", index)
		}
		if hook.OnStart == nil && hook.OnStop == nil {
			return nil, fmt.Errorf("lifecycle: hook %q has no callbacks", hook.Name)
		}
	}
	return &Manager{
		hooks: append([]Hook(nil), hooks...),
		done:  make(chan struct{}),
	}, nil
}

// Start invokes hooks in registration order. A failure stops only the hooks
// that started successfully and returns both the start and rollback errors.
func (manager *Manager) Start(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	manager.mu.Lock()
	if manager.state != stateNew {
		currentState := manager.state
		manager.mu.Unlock()
		return fmt.Errorf("%w: start from state %d", ErrInvalidState, currentState)
	}
	manager.state = stateStarting
	hooks := append([]Hook(nil), manager.hooks...)
	manager.mu.Unlock()

	started := make([]Hook, 0, len(hooks))
	for _, hook := range hooks {
		if hook.OnStart != nil {
			if err := hook.OnStart(ctx); err != nil {
				startErr := fmt.Errorf("start %q: %w", hook.Name, err)
				rollbackErr := stopHooks(ctx, started)

				manager.mu.Lock()
				manager.state = stateStopped
				manager.started = nil
				close(manager.done)
				manager.mu.Unlock()
				return errors.Join(startErr, rollbackErr)
			}
		}
		started = append(started, hook)
	}

	manager.mu.Lock()
	manager.started = started
	manager.state = stateStarted
	manager.mu.Unlock()
	return nil
}

// Stop invokes all successfully started hooks in reverse order. It is
// idempotent and returns the same joined error after the first completed stop.
func (manager *Manager) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}

	manager.mu.Lock()
	switch manager.state {
	case stateNew:
		manager.state = stateStopped
		close(manager.done)
		manager.mu.Unlock()
		return nil
	case stateStarting:
		manager.mu.Unlock()
		return fmt.Errorf("%w: stop while starting", ErrInvalidState)
	case stateStarted:
		manager.state = stateStopping
		hooks := append([]Hook(nil), manager.started...)
		manager.mu.Unlock()

		stopErr := stopHooks(ctx, hooks)
		manager.mu.Lock()
		manager.stopErr = stopErr
		manager.started = nil
		manager.state = stateStopped
		close(manager.done)
		manager.mu.Unlock()
		return stopErr
	case stateStopping:
		done := manager.done
		manager.mu.Unlock()
		select {
		case <-done:
			manager.mu.Lock()
			defer manager.mu.Unlock()
			return manager.stopErr
		case <-ctx.Done():
			return ctx.Err()
		}
	case stateStopped:
		defer manager.mu.Unlock()
		return manager.stopErr
	default:
		manager.mu.Unlock()
		return ErrInvalidState
	}
}

func stopHooks(ctx context.Context, hooks []Hook) error {
	var stopErr error
	for index := len(hooks) - 1; index >= 0; index-- {
		hook := hooks[index]
		if hook.OnStop == nil {
			continue
		}
		if err := hook.OnStop(ctx); err != nil {
			stopErr = errors.Join(stopErr, fmt.Errorf("stop %q: %w", hook.Name, err))
		}
	}
	return stopErr
}
