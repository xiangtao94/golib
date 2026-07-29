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

清理间隔大于零时会启动过期项 janitor，调用方必须在生命周期结束时调用
`Close`。淘汰回调与清理任务同步执行；回调需要停止当前 cache 时调用非阻塞的
`RequestClose`，不要在回调内调用等待 janitor 结束的 `Close`。运行时 cleanup
只会在 cache 被回收后尽力发送非阻塞停止信号，不能替代显式 `Close`。

`Save` 写出的快照会先记录条目数，再逐条使用 `encoding/gob` 编码；`Load`
在解码条目前校验条目数，避免超容量快照先放大为完整 map。默认字节上限按容量
估算，最小 1 MiB、最大 64 MiB；受信任的大值快照可显式调用 `LoadWithLimit`
提高字节上限，条目数上限仍不可绕过。旧版单 map gob 快照在 4 MiB 以内保持
兼容；更大的旧快照应先用旧版库读取并重新保存为新格式。
