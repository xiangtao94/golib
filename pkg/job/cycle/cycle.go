package cycle

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/xiangtao94/golib/pkg/zlog"
)

// Cycle runs jobs repeatedly. The interval starts after each execution
// completes, so a slow job never overlaps with itself unless Concurrency > 1.
type Cycle struct {
	mu        sync.Mutex
	entries   []*Entry
	beforeRun func(context.Context) bool
	afterRun  func(context.Context)
	cancel    context.CancelFunc
	done      chan struct{}
	running   bool
	wg        sync.WaitGroup
}

type Job interface {
	Run(context.Context) error
}

type Entry struct {
	Interval      time.Duration
	Job           Job
	Concurrency   int
	MaxRetry      int
	RetryInterval time.Duration
}

func New() *Cycle {
	return &Cycle{}
}

type FuncJob func(context.Context) error

func (f FuncJob) Run(ctx context.Context) error {
	return f(ctx)
}

func (c *Cycle) AddBeforeRun(beforeRun func(context.Context) bool) *Cycle {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.beforeRun = beforeRun
	return c
}

func (c *Cycle) AddAfterRun(afterRun func(context.Context)) *Cycle {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.afterRun = afterRun
	return c
}

func (c *Cycle) AddFunc(interval time.Duration, cmd func(context.Context) error) {
	c.AddFuncWithConfig(interval, cmd, 1, 0, time.Second)
}

func (c *Cycle) AddFuncWithConfig(
	interval time.Duration,
	cmd func(context.Context) error,
	concurrency, maxRetry int,
	retryInterval time.Duration,
) {
	if interval <= 0 {
		interval = time.Second
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if maxRetry < 0 {
		maxRetry = 0
	}
	if retryInterval <= 0 {
		retryInterval = time.Second
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		panic("cycle: jobs cannot be added after Start")
	}
	c.entries = append(c.entries, &Entry{
		Interval:      interval,
		Job:           FuncJob(cmd),
		Concurrency:   concurrency,
		MaxRetry:      maxRetry,
		RetryInterval: retryInterval,
	})
}

// Start begins all registered workers. Calling Start while already running is
// an idempotent no-op. Cancellation of parent has the same effect as Stop.
func (c *Cycle) Start(parent context.Context) {
	if parent == nil {
		panic("cycle: nil context")
	}

	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.done = make(chan struct{})
	c.running = true
	done := c.done
	entries := slices.Clone(c.entries)
	for _, entry := range entries {
		for range entry.Concurrency {
			c.wg.Go(func() {
				c.run(runCtx, entry)
			})
		}
	}
	c.mu.Unlock()

	go func() {
		c.wg.Wait()
		c.mu.Lock()
		c.running = false
		c.cancel = nil
		close(done)
		c.mu.Unlock()
	}()
}

// Stop cancels the current run and waits for workers. The wait itself obeys
// ctx, so callers control their shutdown budget.
func (c *Cycle) Stop(ctx context.Context) error {
	if ctx == nil {
		panic("cycle: nil context")
	}

	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	cancel := c.cancel
	done := c.done
	c.mu.Unlock()

	cancel()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Cycle) run(ctx context.Context, entry *Entry) {
	for {
		if ctx.Err() != nil {
			return
		}
		c.runWithRetry(ctx, entry)
		if !wait(ctx, entry.Interval) {
			return
		}
	}
}

func (c *Cycle) runWithRetry(ctx context.Context, entry *Entry) {
	for attempt := 0; attempt <= entry.MaxRetry; attempt++ {
		err := c.runOnce(ctx, entry)
		if err == nil || ctx.Err() != nil {
			return
		}
		zlog.Errorf(ctx, "cycle job failed, retry %d/%d: %+v", attempt+1, entry.MaxRetry, err)
		if attempt == entry.MaxRetry || !wait(ctx, entry.RetryInterval) {
			return
		}
	}
}

func (c *Cycle) runOnce(ctx context.Context, entry *Entry) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("cycle job panic: %v", recovered)
			zlog.Errorf(ctx, "%v\nstack:\n%s", err, debug.Stack())
		}
	}()

	c.mu.Lock()
	beforeRun := c.beforeRun
	afterRun := c.afterRun
	c.mu.Unlock()

	if beforeRun != nil && !beforeRun(ctx) {
		return nil
	}
	if afterRun != nil {
		defer afterRun(ctx)
	}
	return entry.Job.Run(ctx)
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
