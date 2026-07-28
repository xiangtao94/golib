# zlog

`zlog` 是独立 module，只依赖 Zap 与文件 sink，不依赖 Gin、Viper 或根 module。

## 生命周期

```go
config := zlog.DefaultLogConfig()
config.AppName = "users"
config.LogToFile = true
config.LogDir = "/var/log/users"
config.Buffer.Enabled = true

logger, err := zlog.InitLog(config)
if err != nil {
	return err
}
defer zlog.CloseLogger()

logger.Info("started")
```

`InitLog` 应在启动阶段、请求 goroutine 运行前调用。配置会先完整校验和准备，成功后才替换当前 logger；无效 level、format、文件路径或 buffer 组合返回 error。`CloseLogger` flush 并关闭 writer，且可重复调用。

`DefaultLogConfig` 默认输出 JSON 到 stdout，不写文件、不启用 buffer。直接构造 `LogConfig{}` 会得到显式零值输出，因此需要默认行为时应从 `DefaultLogConfig` 开始。

## Context

```go
ctx, requestID := zlog.EnsureRequestID(context.Background(), incomingRequestID)
zlog.Infof(ctx, "request accepted: %s", requestID)

downstream := zlog.WithRequestURI(ctx, "/v1/users")
zlog.InfoLogger(downstream, "calling repository")
```

request ID 只存放在标准 `context.Context`，HTTP header/Gin 的转换由根 module 的 middleware 负责。`WithNoLog` 可对明确的内部请求关闭日志。

文件 sink 使用 lumberjack：单文件 100 MiB、最多 14 个备份、最长 14 天并压缩。它按大小轮转，不保证自然日切割。
