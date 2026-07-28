# zlog

Zap 的上下文日志封装。公共日志函数接收 `context.Context`，Gin 只用于 access-log 专属字段。

```go
ctx := zlog.WithRequestID(context.Background(), "req-123")
zlog.Infof(ctx, "started")
```

来自 Gin 的 request ID 仍兼容；向下游传递统一使用 `X-Request-ID`。

Access log 默认不记录 body 和敏感认证 header。正文捕获由 middleware 的有界长度与 sanitizer 显式控制。

文件 sink 使用 lumberjack，按大小而非按自然日轮转：

- 单文件最大 100 MiB
- 最多 14 个备份
- 最长 14 天
- 压缩旧文件

`CloseLogger` 应在进程退出前调用，以 flush 缓冲日志。
