# golib

面向 Go 1.27 的 Gin 服务基础库。它提供 HTTP 接入、基于标准 `context.Context` 的统一日志关联，以及按需引入的基础设施模块。业务直接组合具体类型并负责资源生命周期；只有真实可替换的边界才定义 Go interface。

一次 HTTP 请求的主要路径是：`gin.Engine` → `middleware.RequestID` → 业务 handler / `web.Handle` → `request.Context()` → `zlog` 与 `httpclient`。`Bootstraps` 安装 request ID；`zlog` 从标准 context 读取它，`httpclient` 将它写入出站 `X-Request-ID`。异步任务和 MCP 工具也使用标准 context，因此业务代码不需要持有 `*gin.Context`。`pkg/otel` 可额外关联 trace/span，但需要应用自己配置 SDK。

## Module

仓库采用 1 个 Core module + 8 个可选 module，并通过 `go.work` 联合开发：

| Module | 用途 |
|---|---|
| `github.com/xiangtao94/golib` | Core：config、lifecycle、health、HTTP server、Web adapter、env、zlog、middleware、HTTP client、job、render |
| `.../pkg/elasticsearch` | Elasticsearch adapter |
| `.../pkg/mcp` | MCP Gin transport |
| `.../pkg/milvus` | Milvus adapter |
| `.../pkg/mongodb` | 官方 MongoDB Go Driver adapter |
| `.../pkg/mysql` | MySQL/GORM adapter |
| `.../pkg/otel` | 可选 OpenTelemetry instrumentation |
| `.../pkg/redis` | Redis adapter |
| `.../pkg/s3` | AWS S3 与 S3-compatible object storage adapter |

业务默认只依赖 Core，并按需增加 adapter。Core 不依赖任何可选 module；需要 Core 能力的 adapter 只单向依赖 Core。`env` 和 `zlog` 虽属于 Core module，但 package 本身不依赖 Gin。`go get` 使用 **module 路径**，代码使用 **package import 路径**；二者在这些子模块上相同。

### 发布契约

根 module 使用 `vX.Y.Z` 标签；每个可选 module 使用其仓库相对路径作为标签前缀，例如 `pkg/otel/vX.Y.Z`。各 module 可以共用版本号，但每个要发布的 module 都必须有自己的标签。发布依赖 Core 的子模块前，先发布 Core，并在子模块 `go.mod` 中要求已经发布的 Core 版本。子模块不能依赖 `v0.0.0` 或提交本地 `replace`。

`go.work` 只用于仓库内联调。CI 对每个可发布 module 使用 `GOWORK=off` 运行检查，以其自身的 `go.mod` 验证外部消费者看到的依赖。`testdata/consumer` 是仅用于测试当前工作树 Core 的本地 fixture，其中的 `replace` 不属于发布产物。

## 业务接入

```bash
go get github.com/xiangtao94/golib@<core-version>

# 按需安装，不使用就不会进入业务 module graph
go get github.com/xiangtao94/golib/pkg/mysql@<adapter-version>
go get github.com/xiangtao94/golib/pkg/mcp@<adapter-version>
go get github.com/xiangtao94/golib/pkg/mongodb@<adapter-version>
go get github.com/xiangtao94/golib/pkg/otel@<instrumentation-version>
go get github.com/xiangtao94/golib/pkg/redis@<adapter-version>
go get github.com/xiangtao94/golib/pkg/s3@<adapter-version>
```

## 启动与关闭

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
	engine := gin.New()
	logConfig := zlog.DefaultLogConfig()
	logConfig.AppName = "users"
	if err := golib.Bootstraps(
		engine,
		golib.WithZlog(logConfig),
		golib.WithAccessLog(),
		golib.WithRecovery(nil),
	); err != nil {
		return err
	}
	defer zlog.CloseLogger()

	engine.GET("/ping", func(c *gin.Context) {
		ctx := c.Request.Context()
		zlog.InfoLogger(ctx, "ping", zlog.String("component", "http"))
		c.JSON(http.StatusOK, gin.H{"request_id": zlog.GetRequestID(ctx)})
	})

	server := golib.NewHTTPServer(engine, golib.DefaultHTTPServerConfig(8080))
	return server.Run(appContext)
}
```

先注册 middleware，再注册路由。`Bootstraps` 总会安装 request ID middleware；传入的其他 option 按顺序安装。访问日志应放在 Recovery 外层，才能记录 Recovery 写出的 500。已有 request ID 会从入站 header 进入请求 context 并写回响应 header，缺失时会生成；当前不校验传入的 ID，公网入口应限制其长度和字符集。业务日志用 `c.Request.Context()` 或其派生 context，出站 Resty 请求需显式 `SetContext(ctx)`，否则无法关联原请求。不要将整个 `*gin.Context` 传给异步任务。

调用方负责 OS signal，并通过取消 `appContext` 触发 HTTP server、cron 和 cycle 的停止。`pprof` 默认不注册；如需启用，只在经过认证且网络隔离的管理 listener 上调用 `RegisterPprof`。

长时间运行的 SSE 或 MCP 流不能使用默认的 30 秒响应写入超时。此类服务可在创建 server 前设置 `serverConfig.WriteTimeout = golib.NoWriteTimeout`，并由 handler、客户端断开事件和应用关闭流程管理连接生命周期。普通 HTTP 服务保留默认超时。

## 业务实现 Web Controller

```go
type CreateUserController struct {
	users *UserApplication
}

func (controller *CreateUserController) Action(
	ctx context.Context,
	request *CreateUserRequest,
) (any, error) {
	return controller.users.Create(ctx, request)
}

controller := &CreateUserController{users: users}
engine.POST("/users", web.Handle[CreateUserRequest](controller))
```

保留 `web.Controller` 是因为它是 Gin adapter 与业务实现之间的真实 seam。Controller 直接注入并复用，请求状态只通过 `Action` 参数传递。固定业务响应包络和通用 DAO 不属于 Core：下游协议由业务类型化 client 拥有，数据库查询由业务 repository 拥有。

## AI / Agent 服务如何组合

| 场景 | 现有能力 | 业务需要负责 |
|---|---|---|
| 模型 API 调用 | `httpclient` 提供超时、显式重试、request ID 与可选 trace 传播 | 供应商 SDK、认证、请求/响应类型、模型错误和限额语义 |
| 实时生成 | `render` 的 SSE 输出、`middleware.UploadEventStream` | 事件协议、取消、心跳和连接结束语义 |
| Agent 工具服务 | 独立 `pkg/mcp` module 将官方 MCP SDK 接入 Gin | 工具 schema、授权、租户与资源范围、工具执行结果 |
| 知识检索 | 按需使用 `pkg/milvus`、`pkg/elasticsearch` 和存储 adapter | 索引 schema、召回排序、访问控制和数据一致性 |
| 后台任务 | `job/cron`、`job/cycle` 与 `lifecycle` | 任务状态、持久化、幂等和恢复策略 |
| 排障与关联 | `zlog` 的 context 字段、可选 `pkg/otel` | trace provider、采样、token/成本指标及敏感内容处理 |

现阶段不新增 AI / Agent module：现有基础能力已覆盖通用传输、日志、任务和存储边界。模型 provider、prompt、tool 执行器、memory 与 Agent 工作流的输入输出契约随业务变化，应由业务服务持有。若多个服务出现相同且稳定的需求，优先评估独立可选 module：模型调用的用量/成本观测、并发与预算控制、流事件规范；先确定跨服务的字段、取消和错误语义，再封装实现。不要为审计单独扩展业务存储，也不要默认记录 prompt、模型回复、工具参数或凭证。

## 安全默认值

- Access log 默认不记录请求/响应 body；启用时必须配置正数上限和 sanitizer。
- HTTP client 不记录 header/body，`RetryCount == 0` 表示不重试；HTTP
  明文地址必须显式开启。
- CORS 必须提供明确 allowlist；rate limit 默认不信任转发头。
- MySQL、MongoDB、Redis、S3 默认要求验证 TLS；明文只可显式开启。
- Prometheus 使用路由模板和状态分类，避免高基数 label。
- 文件日志默认关闭；启用后由 lumberjack 按大小轮转，不承诺自然日切割。

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
  [elasticsearch](pkg/elasticsearch/README.md)、
  [mcp](pkg/mcp/README.md)、
  [milvus](pkg/milvus/README.md)、
  [mongodb](pkg/mongodb/README.md)、
  [mysql](pkg/mysql/README.md)、
  [otel](pkg/otel/README.md)、
  [redis](pkg/redis/README.md)、
  [s3](pkg/s3/README.md)
