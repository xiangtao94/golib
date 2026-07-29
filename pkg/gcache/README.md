# gcache

`gcache` 是一个类型安全的分片内存缓存。每个缓存实例只保存一种值类型，
无需类型断言；分片数用于降低并发写入时的锁竞争。

```go
cache := gcache.New[string](5*time.Minute, 10*time.Minute, 16)
defer cache.Close()

cache.Set("user:42", "active", gcache.DefaultExpiration)
status, found := cache.Get("user:42")
```

`New` 默认最多保留 100,000 条记录；需要更小的内存预算时使用
`NewWithMaxEntries`。容量到达上限后会从目标分片淘汰一条记录，避免 key
持续增长拖垮进程。读取过期项会立即删除该项，即使没有启动 janitor。

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

`Load` 在解码前限制快照字节数，并拒绝条目数超过目标缓存容量的快照，
避免恢复过程绕过运行时容量预算。默认字节上限按容量估算，最小 1 MiB、
最大 64 MiB；受信任的大值快照可显式调用 `LoadWithLimit` 提高字节上限，
条目数上限仍不可绕过。现有 `encoding/gob` 快照格式保持兼容。
