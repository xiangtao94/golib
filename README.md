# golib

面向 Go 1.27 的 Gin 服务基础库。Core 提供应用启动、HTTP 服务、日志、健康检查和资源回收；数据库、对象存储、MCP、AG-UI 等能力按需安装。

## 安装与模块

```bash
go get github.com/xiangtao94/golib@<core-version>
```

按需安装可选模块，例如 `go get github.com/xiangtao94/golib/pkg/agui@<adapter-version>`。

| Module | 用途 |
|---|---|
| `github.com/xiangtao94/golib` | Core：config、env、health、lifecycle、middleware、zlog、HTTP server/client、job、render、web |
| `github.com/xiangtao94/golib/pkg/agui` | AG-UI HTTP/SSE 网关 |
| `github.com/xiangtao94/golib/pkg/elasticsearch` | Elasticsearch |
| `github.com/xiangtao94/golib/pkg/mcp` | MCP Gin transport |
| `github.com/xiangtao94/golib/pkg/milvus` | Milvus |
| `github.com/xiangtao94/golib/pkg/mongodb` | MongoDB |
| `github.com/xiangtao94/golib/pkg/mysql` | MySQL |
| `github.com/xiangtao94/golib/pkg/otel` | OpenTelemetry |
| `github.com/xiangtao94/golib/pkg/redis` | Redis |
| `github.com/xiangtao94/golib/pkg/s3` | S3 兼容对象存储 |

## 创建应用

### 直接创建 App（默认端口 8080）

```go
package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func run(appContext context.Context) error {
	app, err := golib.NewApp("users")
	if err != nil {
		return err
	}
	app.GET("/ping", func(c *gin.Context) {
		ctx := c.Request.Context()
		zlog.InfoLogger(ctx, "ping", zlog.String("component", "http"))
		c.JSON(http.StatusOK, gin.H{"request_id": zlog.GetRequestID(ctx)})
	})
	return app.Run(appContext)
}
```

### 自定义选项与端口

```go
app, err := golib.NewApp("users",
	golib.WithAccessLog(),
	golib.WithPrometheus(),
)
if err != nil {
	return err
}
app.GET("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })

config := golib.DefaultHTTPServerConfig(9000)
return app.RunWithConfig(appContext, config)
```

### 使用已有 Gin engine

```go
engine := gin.New()
app, err := golib.Bootstraps(engine, "users", golib.WithAccessLog())
if err != nil {
	return err
}
engine.GET("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })
return app.Run(appContext)
```

在注册业务路由前调用 `Bootstraps`。使用 `gin.New()`；`gin.Default()` 已安装 Logger 和 Recovery。

### 使用自定义 HTTP server

将 Gin 路由挂在 `/api` 下：

```go
app, err := golib.NewApp("users")
if err != nil {
	return err
}
app.GET("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })

handler := http.StripPrefix("/api", app.Engine)
server := golib.NewHTTPServer(handler, golib.DefaultHTTPServerConfig(9000))
return app.RunServer(appContext, server)
```

业务路由为 `/api/ping`，健康检查为 `/api/livez`。AG-UI 长连接参见 [AG-UI 接入示例](pkg/agui/README.md)。

## 默认能力与关闭

- `NewApp` 和 `Bootstraps` 要求应用名，默认初始化 zlog、request ID、Recovery 和 `GET /livez`。`WithZlog` 用于自定义日志配置；应用名以创建参数为准。
- `gate := health.New()` 配合 `WithReadiness(gate)` 注册 `GET /readyz`。资源准备完成后调用 `gate.SetReady()`，停止接收流量前调用 `gate.SetNotReady()`。
- `WithAccessLog()` 启用访问日志；`WithPrometheus()` 注册 `/metrics`；`WithRecovery(handler)` 自定义 panic 处理。
- 业务日志使用 `c.Request.Context()`；出站 Resty 请求使用 `SetContext(ctx)` 传递 request ID。
- 调用方处理 OS signal 并取消传给 `Run` 的 context。`Run`、`RunWithConfig`、`RunServer` 会先关闭 HTTP server，再按登记顺序的逆序执行 `AddStop` 回调，最后关闭日志。启动准备阶段可能提前返回时，在创建 `App` 后使用 `defer app.Stop(context.Background())`。
- 长连接服务需按场景设置 `config.WriteTimeout = golib.NoWriteTimeout`，并在 handler 中处理客户端断开。`pprof` 需显式调用 `RegisterPprof`，仅挂载到受保护的管理入口。

## 文档

- [模块划分与依赖方向](ARCHITECTURE.md)
- Core package：
  [config](pkg/config/README.md)、
  [env](pkg/env/README.md)、
  [errors](pkg/errors/README.md)、
  [gcache](pkg/gcache/README.md)、
  [health](pkg/health/README.md)、
  [httpclient](pkg/httpclient/README.md)、
  [job](pkg/job/README.md)、
  [lifecycle](pkg/lifecycle/README.md)、
  [middleware](pkg/middleware/README.md)、
  [render](pkg/render/README.md)、
  [web](pkg/web/README.md)、
  [zlog](pkg/zlog/README.md)
- 可选 module：
  [agui](pkg/agui/README.md)、
  [elasticsearch](pkg/elasticsearch/README.md)、
  [mcp](pkg/mcp/README.md)、
  [milvus](pkg/milvus/README.md)、
  [mongodb](pkg/mongodb/README.md)、
  [mysql](pkg/mysql/README.md)、
  [otel](pkg/otel/README.md)、
  [redis](pkg/redis/README.md)、
  [s3](pkg/s3/README.md)
