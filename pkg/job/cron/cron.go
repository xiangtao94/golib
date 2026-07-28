package cron

import (
	"context"
	"log"
	"os"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/xiangtao94/golib/pkg/zlog"
)

// Cron keeps track of scheduled entries. Its lifecycle is owned by the caller
// through context cancellation or Stop; it does not own OS signals.
type Cron struct {
	mu        sync.RWMutex
	entries   []*Entry
	running   bool
	cancel    context.CancelFunc
	done      chan struct{}
	wake      chan struct{}
	jobs      sync.WaitGroup
	ErrorLog  *log.Logger
	location  *time.Location
	beforeRun func(context.Context) bool
	afterRun  func(context.Context)
}

type Job interface {
	Run(context.Context) error
}

type Schedule interface {
	Next(time.Time) time.Time
}

type Entry struct {
	Schedule Schedule
	Next     time.Time
	Prev     time.Time
	Job      Job
	Spec     string
}

type byTime []*Entry

func (entries byTime) Len() int      { return len(entries) }
func (entries byTime) Swap(i, j int) { entries[i], entries[j] = entries[j], entries[i] }
func (entries byTime) Less(i, j int) bool {
	if entries[i].Next.IsZero() {
		return false
	}
	if entries[j].Next.IsZero() {
		return true
	}
	return entries[i].Next.Before(entries[j].Next)
}

func New() *Cron {
	return NewWithLocation(time.Now().Location())
}

func NewWithLocation(location *time.Location) *Cron {
	if location == nil {
		location = time.Local
	}
	return &Cron{
		wake:     make(chan struct{}, 1),
		ErrorLog: log.New(os.Stderr, "\n\n\u001B[31m", 0),
		location: location,
	}
}

type FuncJob func(context.Context) error

func (f FuncJob) Run(ctx context.Context) error { return f(ctx) }

func (c *Cron) AddBeforeRun(beforeRun func(context.Context) bool) *Cron {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.beforeRun = beforeRun
	return c
}

func (c *Cron) AddAfterRun(afterRun func(context.Context)) *Cron {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.afterRun = afterRun
	return c
}

func (c *Cron) AddFunc(spec string, cmd func(context.Context) error) error {
	return c.AddJob(spec, FuncJob(cmd))
}

func (c *Cron) AddJob(spec string, cmd Job) error {
	schedule, err := Parse(spec)
	if err != nil {
		return err
	}
	c.Schedule(spec, schedule, cmd)
	return nil
}

func (c *Cron) Schedule(spec string, schedule Schedule, cmd Job) {
	entry := &Entry{Schedule: schedule, Job: cmd, Spec: spec}

	c.mu.Lock()
	if c.running {
		entry.Next = schedule.Next(c.now())
	}
	c.entries = append(c.entries, entry)
	c.mu.Unlock()
	c.signalWake()
}

func (c *Cron) Entries() []*Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entrySnapshotLocked()
}

func (c *Cron) Location() *time.Location {
	return c.location
}

// Start runs the scheduler asynchronously. A repeated Start is an idempotent
// no-op. The scheduler can be started again after a completed Stop.
func (c *Cron) Start(parent context.Context) {
	if parent == nil {
		parent = context.Background()
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
	now := c.now()
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
	}
	done := c.done
	c.mu.Unlock()

	go c.run(runCtx, done)
}

// Run starts the scheduler and blocks until it has stopped.
func (c *Cron) Run(ctx context.Context) {
	c.Start(ctx)
	c.mu.RLock()
	done := c.done
	c.mu.RUnlock()
	if done != nil {
		<-done
	}
}

func (c *Cron) runWithRecovery(ctx context.Context, entry *Entry) {
	defer c.jobs.Done()
	defer func() {
		if recovered := recover(); recovered != nil {
			zlog.Errorf(ctx, "cron job panic: %v\nstack:\n%s", recovered, debug.Stack())
		}
	}()

	c.mu.RLock()
	beforeRun := c.beforeRun
	afterRun := c.afterRun
	c.mu.RUnlock()
	if beforeRun != nil && !beforeRun(ctx) {
		return
	}
	if afterRun != nil {
		defer afterRun(ctx)
	}
	if err := entry.Job.Run(ctx); err != nil {
		zlog.Errorf(ctx, "failed to run cron job: %+v", err)
	}
}

func (c *Cron) run(ctx context.Context, done chan struct{}) {
	defer func() {
		c.jobs.Wait()
		c.mu.Lock()
		c.running = false
		c.cancel = nil
		close(done)
		c.mu.Unlock()
	}()

	for {
		waitDuration := c.nextWaitAndRunDue(ctx)
		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			stopTimer(timer)
			return
		case <-c.wake:
			stopTimer(timer)
		case <-timer.C:
		}
	}
}

func (c *Cron) nextWaitAndRunDue(ctx context.Context) time.Duration {
	now := c.now()

	c.mu.Lock()
	sort.Sort(byTime(c.entries))
	for _, entry := range c.entries {
		if entry.Next.IsZero() || entry.Next.After(now) {
			break
		}
		entry.Prev = entry.Next
		entry.Next = entry.Schedule.Next(now)
		c.jobs.Add(1)
		go c.runWithRecovery(ctx, entry)
	}
	sort.Sort(byTime(c.entries))
	if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
		c.mu.Unlock()
		return 24 * time.Hour
	}
	waitDuration := time.Until(c.entries[0].Next)
	c.mu.Unlock()
	if waitDuration < 0 {
		return 0
	}
	return waitDuration
}

func (c *Cron) signalWake() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Stop cancels scheduling and waits for the scheduler and in-flight jobs.
func (c *Cron) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.RLock()
	if !c.running {
		c.mu.RUnlock()
		return nil
	}
	cancel := c.cancel
	done := c.done
	c.mu.RUnlock()
	cancel()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (c *Cron) entrySnapshotLocked() []*Entry {
	entries := make([]*Entry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, &Entry{
			Schedule: entry.Schedule,
			Next:     entry.Next,
			Prev:     entry.Prev,
			Job:      entry.Job,
			Spec:     entry.Spec,
		})
	}
	return entries
}

func (c *Cron) now() time.Time {
	return time.Now().In(c.location)
}

func (c *Cron) logf(format string, args ...interface{}) {
	if c.ErrorLog != nil {
		c.ErrorLog.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}
