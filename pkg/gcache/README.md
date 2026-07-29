# gcache

`gcache` 是一个类型安全的分片内存缓存。每个缓存实例只保存一种值类型，
无需类型断言；分片数用于降低并发写入时的锁竞争。

```go
cache := gcache.New[string](5*time.Minute, 10*time.Minute, 16)
defer cache.Close()

cache.Set("user:42", "active", gcache.DefaultExpiration)
status, found := cache.Get("user:42")
```

需要原子读改写时使用 `Update`，它替代了按每种数字类型重复定义的
`IncrementInt`、`IncrementFloat64` 等接口：

```go
counts := gcache.New[int](gcache.NoExpiration, 0, 16)
counts.Set("requests", 0, gcache.DefaultExpiration)
next, err := counts.Update("requests", func(current int) (int, error) {
	return current + 1, nil
})
```

清理间隔大于零时会启动过期项 janitor。正常生命周期中应调用 `Close`；
Go 1.26 的 `runtime.AddCleanup` 仅作为遗漏关闭时的兜底。
