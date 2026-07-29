# Job

`cron` 和 `cycle` 都由调用方 context 控制生命周期，不处理 OS signal，也不持有 Gin context。

## Cron

Cron 按 schedule 触发；同一 entry 的前一次执行未完成时会合并后续 tick，不会与自身重叠。

```go
scheduler := cron.New()
if err := scheduler.AddFunc("*/5 * * * * *", func(ctx context.Context) error {
	return refresh(ctx)
}); err != nil {
	return err
}
scheduler.Start(appContext)

shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
return scheduler.Stop(shutdownCtx)
```

`Start` 对同一轮运行幂等，`Stop` 等待 scheduler 和已启动 job。停止后可再次 `Start`。

## Cycle

Cycle 在一次执行完成后再等待 interval，因此默认不会与自身重叠：

```go
worker := cycle.New()
worker.AddFuncWithConfig(
	time.Minute,
	func(ctx context.Context) error { return syncData(ctx) },
	1,
	3,
	time.Second,
)
worker.Start(appContext)
defer worker.Stop(shutdownContext)
```

job 必须响应 `ctx.Done()`。`Cycle` 启动后不允许继续添加 job；panic 会恢复并记录，error 按显式重试配置处理。
