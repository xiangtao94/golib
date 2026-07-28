# golib

面向 Go 1.26 的后端基础库。设计目标是：业务直接组合具体类型，生命周期由调用方持有，只有真实可替换的 seam 才定义 Go interface。

## Module

仓库采用 1 个 Core module + 6 个可选 adapter module，并通过 `go.work` 联合开发：

| Module | Interface |
|---|---|
| `github.com/xiangtao94/golib` | Core：HTTP server、Flow、env、zlog、middleware、HTTP client、job、render |
| `.../pkg/elasticsearch` | Elasticsearch adapter |
| `.../pkg/mcp` | MCP Gin transport |
| `.../pkg/milvus` | Milvus adapter |
| `.../pkg/orm` | MySQL/GORM adapter |
| `.../pkg/oss` | MinIO adapter |
| `.../pkg/redis` | Redis adapter |

业务默认只依赖 Core，并按需增加 adapter。Core 不依赖任何可选 adapter；adapter 只单向依赖 Core 中稳定的 context 日志与配置 package。`env` 和 `zlog` 虽属于 Core module，但 package 本身不依赖 Gin。

## 业务接入

```bash
go get github.com/xiangtao94/golib@<core-version>

# 按需安装，不使用就不会进入业务 module graph
go get github.com/xiangtao94/golib/pkg/orm@<adapter-version>
go get github.com/xiangtao94/golib/pkg/redis@<adapter-version>
```

业务代码按 package import：

```go
import (
	"github.com/xiangtao94/golib/flow"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/zlog"

	"github.com/xiangtao94/golib/pkg/orm"
)
```

仓库内 adapter `go.mod` 的本地 `replace github.com/xiangtao94/golib => ../..` 只用于关闭 `go.work` 后的独立测试。下游业务不会继承依赖 module 的 `replace`。发布 adapter 前，将其 Core 占位版本更新为已发布的 Core 版本。

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

## 业务实现 Flow interface

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

engine.POST("/users", flow.Use(func() flow.Controller[CreateUserRequest] {
	return &CreateUserController{users: users}
}))
```

保留 `flow.Controller` 是因为它是 Gin adapter 与业务实现之间的真实 seam；旧的 `IApi`、`IDao`、`ILayer`、`IService`、`IData` 已删除。DAO、外部 HTTP client 和 registry 使用具体类型组合。

## 安全默认值

- Access log 默认不记录请求/响应 body；启用时必须配置正数上限和 sanitizer。
- HTTP client 的 body 日志默认关闭，`RetryCount == 0` 表示不重试。
- CORS 必须提供明确 allowlist；rate limit 默认不信任转发头。
- MySQL、Redis、MinIO 默认要求验证 TLS；明文只可显式开启。
- Prometheus 使用路由模板和状态分类，避免高基数 label。
- 文件日志默认关闭；启用后由 lumberjack 按大小轮转，不承诺自然日切割。

## 文档

- [架构与 interface 归属](ARCHITECTURE.md)
- [破坏性迁移说明](MIGRATION.md)
- [Flow](flow/README.md)
- [HTTP client](pkg/http/README.md)

## 验证

测试与源码同 package 放置为 `*_test.go`。只有 fixture 放入 `testdata/`；不建立顶层 `tests/`，避免测试跨 module 导入内部实现。

```bash
modules=(. pkg/elasticsearch pkg/mcp pkg/milvus pkg/orm pkg/oss pkg/redis)
for module in "${modules[@]}"; do
	(cd "$module" && GOWORK=off go test -race ./... && GOWORK=off go vet ./...)
done
```
