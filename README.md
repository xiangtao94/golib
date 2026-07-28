# golib

面向 Go 1.26 的后端基础库。当前版本采用安全默认值、显式依赖注入和调用方控制的生命周期；Gin 只保留在 HTTP adapter，不再作为业务层、存储层或后台任务的上下文类型。

## 模块结构

仓库使用 `go.work` 联合开发，重依赖 adapter 已拆成独立 module：

| Module | 用途 |
|---|---|
| `github.com/xiangtao94/golib` | HTTP server、Flow、middleware、日志、缓存、通用 HTTP client |
| `.../pkg/elasticsearch` | Elasticsearch v9 |
| `.../pkg/mcp` | 官方 MCP SDK 的 Gin transport |
| `.../pkg/milvus` | Milvus v2 client |
| `.../pkg/orm` | MySQL/GORM adapter |
| `.../pkg/oss` | MinIO adapter |
| `.../pkg/redis` | Redis adapter |

业务只需依赖实际使用的 module，接入 Redis 不会再被迫下载 Milvus、Elasticsearch 等依赖。

## HTTP server

```go
engine := gin.New()
if err := golib.Bootstraps(
    engine,
    golib.WithRecovery(nil),
    golib.WithAccessLog(),
); err != nil {
    return err
}

server := golib.NewHTTPServer(engine, golib.DefaultHTTPServerConfig(8080))
return server.Run(appContext)
```

调用方负责 OS signal，并通过取消 `appContext` 触发优雅关闭。默认配置包含 header/read/write/idle/shutdown timeout 和 header 大小上限。

`pprof` 默认不注册。如确需启用，使用 `golib.RegisterPprof` 挂载到经过认证且网络隔离的管理端口。

## Flow

Controller 通过 factory 显式创建，依赖不会在反射复制时丢失：

```go
engine.POST("/users", flow.Use(func() flow.IController[CreateUserRequest] {
    return NewCreateUserController(userService)
}))
```

业务层统一接收 `context.Context`。其他 layer 使用 factory 创建：

```go
service := flow.Create(ctx, func() *UserService {
    return NewUserService(repository)
})
```

数据库通过实例 `flow.DBRegistry` 注入，不存在包级默认 DB。

## 中间件安全默认值

- Access log 默认不记录请求/响应正文。启用正文捕获时必须同时提供有界长度和 sanitizer。
- Rate limit 默认使用 TCP peer 地址，不信任转发头；只有 Gin trusted proxies 配置正确时才使用 `TrustedProxyClientIP`。
- CORS 必须提供明确的 origin allowlist；`*` 会在构造时失败。
- Prometheus 使用路由模板和 `2xx/4xx/5xx` 标签，避免用户 ID、原始 URL 和精确状态造成高基数。
- Gzip 自动写入 `Vary: Accept-Encoding`，支持 Flush，并跳过 SSE。
- Timeout 是协作式取消：handler 必须监听 `ctx.Done()`；中间件不会并发操作 Gin writer，也不会假装能强杀副作用。

## 外部连接

MySQL、Redis 和 MinIO 默认要求验证过的 TLS。仅本地开发确需明文时，显式设置对应的 `AllowInsecureTransport`：

```go
mysqlConf := orm.MysqlConf{TLSConfigName: "true"}
redisConf := redis.RedisConf{TLSConfig: &tls.Config{ServerName: "redis.example.com"}}
minioConf := oss.MinioConf{Endpoint: "https://storage.example.com"}
```

## 日志轮转

文件日志使用 lumberjack，当前语义是按大小轮转：单文件 100 MiB、最多 14 个备份、最多保留 14 天并压缩。它不是“每天零点切文件”。如业务要求严格按自然日切割，应在进程外使用日志收集器或提供独立的 time-based sink。

## 本地验证

```bash
go test ./...
go test -race ./...
go vet ./...

for module in pkg/elasticsearch pkg/mcp pkg/milvus pkg/orm pkg/oss pkg/redis; do
  (cd "$module" && GOWORK=off go test ./... && GOWORK=off go test -race ./... && GOWORK=off go vet ./...)
done
```

## 迁移摘要

- `flow.Use(&Controller{})` → `flow.Use(func() IController[T] { ... })`
- `Action(*T)` → `Action(context.Context, *T)`
- `flow.Create(ctx, &Layer{})` → `flow.Create(ctx, factory)`
- `SetDefaultDBClient/SetNamedDBClient` → 注入 `flow.NewDBRegistry`
- `RegisterRender` → 注入 `render.NewJSONRenderer(factory)`
- Cron/Cycle 的 `Start()`/`Stop()` → `Start(ctx)`/`Stop(ctx)`
- `RateLimitMiddleware(rate, burst, ttl)` → `RateLimitMiddleware(RateLimiterConfig{...})`
- `Cors` → `NewCORS(CORSConfig{...})`
- Milvus `SearchVectors(..., outputFields)` → `SearchVectors(..., SearchOptions{...})`
