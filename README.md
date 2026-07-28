# golib

面向 Go 1.26 的后端基础库。设计目标是：业务直接组合具体类型，生命周期由调用方持有，只有真实可替换的 seam 才定义 Go interface。

## Module

仓库采用 1 个 Core module + 8 个可选 module，并通过 `go.work` 联合开发：

| Module | Interface |
|---|---|
| `github.com/xiangtao94/golib` | Core：config、lifecycle、health、HTTP server、Web adapter、env、zlog、middleware、HTTP client、job、render |
| `.../pkg/elasticsearch` | Elasticsearch adapter |
| `.../pkg/mcp` | MCP Gin transport |
| `.../pkg/milvus` | Milvus adapter |
| `.../pkg/mongodb` | 官方 MongoDB Go Driver adapter |
| `.../pkg/orm` | MySQL/GORM adapter |
| `.../pkg/otel` | 可选 OpenTelemetry instrumentation |
| `.../pkg/redis` | Redis adapter |
| `.../pkg/s3` | AWS S3 与 S3-compatible object storage adapter |

业务默认只依赖 Core，并按需增加 adapter。Core 不依赖任何可选 adapter；adapter 只单向依赖 Core 中稳定的 context 日志与配置 package。`env` 和 `zlog` 虽属于 Core module，但 package 本身不依赖 Gin。

## 业务接入

```bash
go get github.com/xiangtao94/golib@<core-version>

# 按需安装，不使用就不会进入业务 module graph
go get github.com/xiangtao94/golib/pkg/orm@<adapter-version>
go get github.com/xiangtao94/golib/pkg/mongodb@<adapter-version>
go get github.com/xiangtao94/golib/pkg/otel@<instrumentation-version>
go get github.com/xiangtao94/golib/pkg/redis@<adapter-version>
go get github.com/xiangtao94/golib/pkg/s3@<adapter-version>
```

业务代码按 package import：

```go
import (
	"github.com/xiangtao94/golib/pkg/config"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/health"
	"github.com/xiangtao94/golib/pkg/httpclient"
	"github.com/xiangtao94/golib/pkg/lifecycle"
	"github.com/xiangtao94/golib/pkg/web"
	"github.com/xiangtao94/golib/pkg/zlog"

	"github.com/xiangtao94/golib/pkg/mongodb"
	"github.com/xiangtao94/golib/pkg/orm"
)
```

## 启动与关闭

```go
engine := gin.New()

logConfig := zlog.DefaultLogConfig()
logConfig.AppName = "users"
if err := golib.Bootstraps(
	engine,
	golib.WithZlog(logConfig),
	golib.WithRecovery(nil),
	golib.WithAccessLog(),
); err != nil {
	return err
}
defer zlog.CloseLogger()

server := golib.NewHTTPServer(engine, golib.DefaultHTTPServerConfig(8080))
return server.Run(appContext)
```

`Bootstraps` 总会安装 request ID middleware。调用方负责 OS signal，并通过取消 `appContext` 触发 HTTP server、cron 和 cycle 的停止。`pprof` 默认不注册；如需启用，只在经过认证且网络隔离的管理 listener 上调用 `RegisterPprof`。

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
  [orm](pkg/orm/README.md)、
  [otel](pkg/otel/README.md)、
  [redis](pkg/redis/README.md)、
  [s3](pkg/s3/README.md)
