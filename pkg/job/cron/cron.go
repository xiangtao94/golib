package cron

import (
	"log"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/xiangtao94/golib/pkg/zlog"

	"github.com/gin-gonic/gin"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	entries   []*Entry
	stop      chan struct{}
	add       chan *Entry
	snapshot  chan []*Entry
	running   bool
	ErrorLog  *log.Logger
	location  *time.Location
	gin       *gin.Engine
	beforeRun func(*gin.Context) bool
	afterRun  func(*gin.Context)
}

// Job is an interface for submitted cron jobs.
type Job interface {
	Run(ctx *gin.Context) error
}

// The Schedule describes a job's duty cycle.
type Schedule interface {
	// Return the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// Entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	// The schedule on which this job should be run.
	Schedule Schedule

	// The next time the job will run. This is the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next time.Time

	// The last time this job was run. This is the zero time if the job has never
	// been run.
	Prev time.Time

	// The Job to run.
	Job Job

	//spec
	Spec string
}

// byTime is a wrapper for sorting the entry array by time
// (with zero time at the end).
type byTime []*Entry

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}

// New returns a new Cron job runner, in the Local time zone.
func New(engine *gin.Engine) *Cron {
	return NewWithLocation(time.Now().Location(), engine)
}

// NewWithLocation returns a new Cron job runner.
func NewWithLocation(location *time.Location, engine *gin.Engine) *Cron {
	return &Cron{
		entries:  nil,
		add:      make(chan *Entry),
		stop:     make(chan struct{}),
		snapshot: make(chan []*Entry),
		running:  false,
		ErrorLog: log.New(os.Stderr, "\n\n\u001B[31m", 0),
		location: location,
		gin:      engine,
	}
}

// A wrapper that turns a func() into a cron.Job
type FuncJob func(*gin.Context) error

func (f FuncJob) Run(ctx *gin.Context) error { return f(ctx) }

// add cron before func
func (c *Cron) AddBeforeRun(beforeRun func(*gin.Context) bool) *Cron {
	c.beforeRun = beforeRun
	return c
}

// add cron after func
func (c *Cron) AddAfterRun(afterRun func(*gin.Context)) *Cron {
	c.afterRun = afterRun
	return c
}

// AddFunc adds a func to the Cron to be run on the given schedule.
func (c *Cron) AddFunc(spec string, cmd func(*gin.Context) error) error {
	return c.AddJob(spec, FuncJob(cmd))
}

// AddJob adds a Job to the Cron to be run on the given schedule.
func (c *Cron) AddJob(spec string, cmd Job) error {
	schedule, err := Parse(spec)
	if err != nil {
		return err
	}
	c.Schedule(spec, schedule, cmd)
	return nil
}

// Schedule adds a Job to the Cron to be run on the given schedule.
func (c *Cron) Schedule(spec string, schedule Schedule, cmd Job) {
	entry := &Entry{
		Schedule: schedule,
		Job:      cmd,
		Spec:     spec,
	}
	if !c.running {
		c.entries = append(c.entries, entry)
		return
	}

	c.add <- entry
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []*Entry {
	if c.running {
		c.snapshot <- nil
		x := <-c.snapshot
		return x
	}
	return c.entrySnapshot()
}

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
}

// Start the cron scheduler in its own go-routine, or no-op if already started.
func (c *Cron) Start() {
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

// Run the cron scheduler, or no-op if already running.
func (c *Cron) Run() {
	if c.running {
		return
	}
	c.running = true
	c.run()
}

func (c *Cron) runWithRecovery(e *Entry) {
	ctx := gin.CreateTestContextOnly(nil, c.gin)

	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]

			// 使用现有的zlog API记录错误
			zlog.Errorf(ctx, "cron job panic: %+v\nhandle: %s\nrequestId: %s\nlogId: %s\nstack:\n%s",
				r,
				ctx.HandlerName(),
				ctx.GetString("requestId"),
				ctx.GetString("logID"),
				string(buf),
			)
		}
	}()

	if c.beforeRun != nil {
		ok := c.beforeRun(ctx)
		if !ok {
			return
		}
	}

	err := e.Job.Run(ctx)
	if err != nil {
		zlog.Errorf(ctx, "failed to run cron job: %+v", err)
	}
	if c.afterRun != nil {
		c.afterRun(ctx)
	}
}

// Run the scheduler. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
	// Figure out the next activation times for each entry.
	now := c.now()
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
	}

	for {
		// Determine the next entry to run.
		sort.Sort(byTime(c.entries))

		var timer *time.Timer
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			timer = time.NewTimer(100000 * time.Hour)
		} else {
			timer = time.NewTimer(c.entries[0].Next.Sub(now))
		}

		for {
			select {
			case now = <-timer.C:
				now = now.In(c.location)
				// Run every entry whose next time was less than now
				for _, e := range c.entries {
					if e.Next.After(now) || e.Next.IsZero() {
						break
					}
					go c.runWithRecovery(e)
					e.Prev = e.Next
					e.Next = e.Schedule.Next(now)
				}

			case newEntry := <-c.add:
				timer.Stop()
				now = c.now()
				newEntry.Next = newEntry.Schedule.Next(now)
				c.entries = append(c.entries, newEntry)

			case <-c.snapshot:
				c.snapshot <- c.entrySnapshot()
				continue

			case <-c.stop:
				timer.Stop()
				return
			}

			break
		}
	}
}

// Logs an error to stderr or to the configured error log
func (c *Cron) logf(format string, args ...interface{}) {
	if c.ErrorLog != nil {
		c.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Stop stops the cron scheduler if it is running; otherwise it does nothing.
func (c *Cron) Stop() {
	if !c.running {
		return
	}
	c.stop <- struct{}{}
	c.running = false
}

// entrySnapshot returns a copy of the current cron entry list.
func (c *Cron) entrySnapshot() []*Entry {
	entries := []*Entry{}
	for _, e := range c.entries {
		entries = append(entries, &Entry{
			Schedule: e.Schedule,
			Next:     e.Next,
			Prev:     e.Prev,
			Job:      e.Job,
		})
	}
	return entries
}

// now returns current time in c location
func (c *Cron) now() time.Time {
	return time.Now().In(c.location)
}
