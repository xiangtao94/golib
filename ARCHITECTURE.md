# Architecture

## Module 与依赖方向

```text
业务 module
  ├─> Core module（config / lifecycle / health / web / env / zlog / httpclient / middleware / job / render）
  └─> 按需 adapter module（orm / mongodb / redis / s3 / milvus / elasticsearch / mcp）
           └─> Core 中的 env / zlog package
  └─> 按需 instrumentation module（otel）
           └─> Core 中的 httpclient / zlog package

Core ─X─> 可选 adapter
Core ─X─> 可选 instrumentation
zlog ─> Zap
config ─> Viper
```

根 module 是统一发布的 Core。`config`、`env`、`zlog`、`lifecycle`、`health` 作为 Core package 保持框架无关；重型 adapter 和 instrumentation 使用独立 module 隔离 SDK 依赖，并且只能单向依赖 Core。这样 N 个业务只学习一套 Core contract，同时不会因未使用 Milvus、Elasticsearch、OpenTelemetry 等能力而引入它们。

## 保留的 interface

interface 只放在使用它的 seam，不为“将来可能替换”预建同形抽象：

| Interface | Owner | 为什么保留 |
|---|---|---|
| `web.Controller[T]` | `web` Gin adapter | 多个业务 Controller 直接实现，adapter 统一绑定、context 和渲染 |
| `cron.Job` / `cycle.Job` | scheduler | `FuncJob` 与业务 struct 都是实际 adapter |
| `cron.Schedule` | scheduler | cron spec 与 constant-delay 已有两个实现 |

旧的 `flow` package 混合了三个无关 seam，因此已拆除：Controller adapter 移到 `pkg/web`；固定业务响应包络交还业务 client；通用 GORM DAO 删除并由业务 repository 持有查询语义。Core 因此不再依赖 GORM。

错误响应通过函数类型 `render.Factory` 注入，不为单方法回调额外定义 interface。业务自己的模型 provider、tool executor、memory、repository interface 应由业务 consumer 定义；只有出现第二个 adapter 或明确测试 seam 时再引入。

## 并发与生命周期 owner

| 资源 | 创建 owner | 关闭 owner |
|---|---|---|
| HTTP server | 应用入口 | 取消 app context，server 完成 graceful shutdown |
| Resty HTTP client | 调用它的业务 module | `resty.Client.Close` |
| Logger | 应用入口 | `zlog.CloseLogger` |
| Cron/Cycle | 应用入口或业务 runtime | 取消 parent context 或带 deadline 的 `Stop` |
| OpenTelemetry SDK/provider/exporter | 应用入口 | SDK shutdown |
| DB/Redis/外部 adapter | 业务基础设施组装层 | 同一组装层 |

库不捕获 OS signal，不在 GET 中写状态，不持有包级业务连接，也不为了审计扩展业务存储。

## AI agent 中间件决策

当前不引入 Eino、LangChainGo、Genkit 等 agent framework，也不在基础库预定义 model/tool/memory interface。原因是目前只有假设 seam，引入后会让业务受框架类型和生命周期约束。

现阶段可直接复用的真实 seam：

- `context.Context`：取消、deadline、request ID、语言；
- `pkg/httpclient.New` 返回的 `*resty.Client`：模型网关、Resty 原生 SSE、
  幂等重试和连接池；
- `pkg/mcp`：MCP transport；
- `pkg/otel`、`zlog` 与 Prometheus middleware：跨服务 trace、调用链日志和低基数指标；
- cron/cycle：明确 owner 的后台执行。

当业务出现多个模型 provider 或 tool runtime 时，在业务 consumer 侧定义最小 interface。`pkg/otel` 只提供 Gin/HTTP instrumentation 和日志字段关联；SDK、exporter、sampler、provider 与 shutdown 仍由应用入口持有。

## 发布与本地开发

- Core 使用根 module 版本发布，业务直接 `go get github.com/xiangtao94/golib@<version>`。
- adapter 使用各自 module 版本发布，业务只安装实际使用的 adapter。
- 仓库内 `go.work` 负责联合开发；adapter 的本地 `replace` 仅用于 `GOWORK=off` 独立验证。
- 发布 adapter 前，把其 `github.com/xiangtao94/golib v0.0.0` 更新为已发布的 Core 版本。下游不会继承本地 `replace`。

## 测试位置

单元测试和 package 级集成测试与源码同目录，使用 `package x` 或 `package x_test`。fixture 放 `testdata/`。只有真正跨进程、跨 module 的系统测试才建立独立仓库或明确的 system-test module；当前无需顶层 `tests/`。
