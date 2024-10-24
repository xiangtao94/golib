package cycle

import (
	"fmt"
	"github.com/tiant-go/golib/pkg/zlog"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Cycle struct {
	entries   []*Entry
	gin       *gin.Engine
	beforeRun func(*gin.Context) bool
	afterRun  func(*gin.Context)
}

type Job interface {
	Run(ctx *gin.Context) error
}

type Entry struct {
	Duration time.Duration
	Job      Job
}

func New(engine *gin.Engine) *Cycle {
	return &Cycle{
		entries: nil,
		gin:     engine,
	}
}

type FuncJob func(*gin.Context) error

func (f FuncJob) Run(ctx *gin.Context) error { return f(ctx) }

// add cron before func
func (c *Cycle) AddBeforeRun(beforeRun func(*gin.Context) bool) *Cycle {
	c.beforeRun = beforeRun
	return c
}

// add cron after func
func (c *Cycle) AddAfterRun(afterRun func(*gin.Context)) *Cycle {
	c.afterRun = afterRun
	return c
}

func (c *Cycle) AddFunc(duration time.Duration, cmd func(*gin.Context) error) {
	entry := &Entry{
		Duration: duration,
		Job:      FuncJob(cmd),
	}
	c.entries = append(c.entries, entry)
}

func (c *Cycle) Start() {
	for _, e := range c.entries {
		go c.run(e)
	}
}

// 死循环
func (c *Cycle) run(e *Entry) {
	for {
		c.runWithRecovery(e)
	}
}

func (c *Cycle) runWithRecovery(entry *Entry) {
	ctx, _ := gin.CreateTestContext(nil)

	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]

			handleName := ctx.HandlerName()
			requestID := ctx.GetString("requestId")
			logID := ctx.GetString("logID")

			var body strings.Builder
			body.WriteString(`{"level":"ERROR","time":"`)
			body.WriteString(time.Now().Format("2006-01-02 15:04:05.999999"))
			body.WriteString(`","file":"gin/cycle/cycle.go:94","msg":"`)
			body.WriteString(fmt.Sprintf("%+v", r))
			body.WriteString(`","handle":"`)
			body.WriteString(handleName)
			body.WriteString(`","logId":"`)
			body.WriteString(logID)
			body.WriteString(`","requestId":"`)
			body.WriteString(requestID)
			body.WriteString(`","module":"stack"`)
			body.WriteString(`}`)

			std := log.New(os.Stderr, "\n\n\u001B[31m", 0)
			f := "%s\n-------------------stack-start-------------------\n%v\n%s\n-------------------stack-end-------------------\n"
			if std != nil {
				std.Printf(f, body.String(), r, buf)
			} else {
				log.Printf(f, body.String(), r, buf)
			}
		}
		time.Sleep(entry.Duration)
	}()

	if c.beforeRun != nil {
		ok := c.beforeRun(ctx)
		if !ok {
			return
		}
	}

	err := entry.Job.Run(ctx)
	if err != nil {
		zlog.Errorf(ctx, "failed to run cycle job: %+v", err)
	}
	if c.afterRun != nil {
		c.afterRun(ctx)
	}
}
