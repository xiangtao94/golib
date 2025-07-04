package cycle

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type Cycle struct {
	entries   []*Entry
	gin       *gin.Engine
	beforeRun func(*gin.Context) bool
	afterRun  func(*gin.Context)

	cancelFuncs []context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.Mutex
}

type Job interface {
	Run(ctx *gin.Context) error
}

type Entry struct {
	Interval      time.Duration // 任务执行完成后等待多久再次执行
	Job           Job
	Concurrency   int           // 并发数，默认1
	MaxRetry      int           // 失败重试最大次数，默认0不重试
	RetryInterval time.Duration // 重试间隔，默认1秒
}

func New(engine *gin.Engine) *Cycle {
	return &Cycle{
		gin: engine,
	}
}

type FuncJob func(*gin.Context) error

func (f FuncJob) Run(ctx *gin.Context) error {
	return f(ctx)
}

func (c *Cycle) AddBeforeRun(beforeRun func(*gin.Context) bool) *Cycle {
	c.beforeRun = beforeRun
	return c
}

func (c *Cycle) AddAfterRun(afterRun func(*gin.Context)) *Cycle {
	c.afterRun = afterRun
	return c
}

// 新增参数：concurrency 并发数，maxRetry 最大重试次数，retryInterval 重试间隔
func (c *Cycle) AddFunc(interval time.Duration, cmd func(*gin.Context) error) {
	entry := &Entry{
		Interval:      interval,
		Job:           FuncJob(cmd),
		Concurrency:   1,
		MaxRetry:      0,
		RetryInterval: 0,
	}
	c.entries = append(c.entries, entry)
}

// 新增参数：concurrency 并发数，maxRetry 最大重试次数，retryInterval 重试间隔
func (c *Cycle) AddFuncWithConfig(interval time.Duration, cmd func(*gin.Context) error, concurrency, maxRetry int, retryInterval time.Duration) {
	if concurrency <= 0 {
		concurrency = 1
	}
	if retryInterval <= 0 {
		retryInterval = time.Second
	}

	entry := &Entry{
		Interval:      interval,
		Job:           FuncJob(cmd),
		Concurrency:   concurrency,
		MaxRetry:      maxRetry,
		RetryInterval: retryInterval,
	}
	c.entries = append(c.entries, entry)
}

func (c *Cycle) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, e := range c.entries {
		for i := 0; i < e.Concurrency; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			c.cancelFuncs = append(c.cancelFuncs, cancel)
			c.wg.Add(1)
			go c.run(ctx, e)
		}
	}
}

func (c *Cycle) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cancel := range c.cancelFuncs {
		cancel()
	}
	c.cancelFuncs = nil
	c.wg.Wait()
}

func (c *Cycle) run(ctx context.Context, e *Entry) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.runWithRetry(ctx, e)

		select {
		case <-time.After(e.Interval):
		case <-ctx.Done():
			return
		}
	}
}

// 包装了重试逻辑
func (c *Cycle) runWithRetry(ctx context.Context, e *Entry) {
	tryCount := 0
	for {
		err := c.runOnce(ctx, e)
		if err == nil {
			return
		}

		tryCount++
		zlog.Errorf(nil, "cycle job failed, retry %d/%d: %+v", tryCount, e.MaxRetry, err)
		if tryCount > e.MaxRetry {
			return
		}

		select {
		case <-time.After(e.RetryInterval):
		case <-ctx.Done():
			return
		}
	}
}

func (c *Cycle) runOnce(ctx context.Context, e *Entry) error {
	ginCtx := gin.CreateTestContextOnly(nil, c.gin)

	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]

			// 使用现有的zlog API记录错误，性能更好
			zlog.Errorf(ginCtx, "cycle job panic: %+v\nhandle: %s\nrequestId: %s\nlogId: %s\nstack:\n%s",
				r,
				ginCtx.HandlerName(),
				ginCtx.GetString("requestId"),
				ginCtx.GetString("logID"),
				string(buf),
			)
		}
	}()

	if c.beforeRun != nil {
		ok := c.beforeRun(ginCtx)
		if !ok {
			return fmt.Errorf("beforeRun returned false")
		}
	}

	err := e.Job.Run(ginCtx)
	if c.afterRun != nil {
		c.afterRun(ginCtx)
	}
	return err
}
