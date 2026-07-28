# Architecture

## Module 与依赖方向

```text
业务 module
  ├─> golib（HTTP adapter、Flow、middleware、job、render）
  ├─> env
  ├─> zlog
  └─> 按需 adapter（orm / redis / oss / milvus / elasticsearch / mcp）

adapter ─> zlog
redis   ─> env + zlog
zlog    ─> Zap
env     ─> Viper
```

根 module 负责 Gin adapter。`env`、`zlog` 和重型 adapter 不反向依赖根 module，避免基础能力被整个 Web/存储依赖树绑住。

## 保留的 interface

interface 只放在使用它的 seam，不为“将来可能替换”预建同形抽象：

| Interface | Owner | 为什么保留 |
|---|---|---|
| `flow.Controller[T]` | `flow` HTTP adapter | 业务 controller 与 Gin adapter 是两个真实 adapter；业务可直接实现 |
| `cron.Job` / `cycle.Job` | scheduler | `FuncJob` 与业务 struct 都是实际 adapter |
| `cron.Schedule` | scheduler | cron spec 与 constant-delay 已有两个实现 |
| `render.Render` | render adapter | 默认 response 与业务 response shape 可替换 |

`IApi`、`IDao`、`ILayer`、`IService`、`IData` 通过删除测试后只会把同样复杂度搬到调用方，因此已删除。`Api`、`Dao`、`DBRegistry`、HTTP `Client` 都使用具体类型。

业务自己的模型 provider、tool executor、memory、repository interface 应由业务 consumer 定义；只有出现第二个 adapter 或明确测试 seam 时再引入。

## 并发与生命周期 owner

| 资源 | 创建 owner | 关闭 owner |
|---|---|---|
| HTTP server | 应用入口 | 取消 app context，server 完成 graceful shutdown |
| HTTP client | 调用它的业务 module | `Client.Close` |
| Logger | 应用入口 | `zlog.CloseLogger` |
| Cron/Cycle | 应用入口或业务 runtime | 取消 parent context 或带 deadline 的 `Stop` |
| DB/Redis/外部 adapter | 业务基础设施组装层 | 同一组装层 |

库不捕获 OS signal，不在 GET 中写状态，不持有包级业务连接，也不为了审计扩展业务存储。

## AI agent 中间件决策

当前不引入 Eino、LangChainGo、Genkit 等 agent framework，也不在基础库预定义 model/tool/memory interface。原因是目前只有假设 seam，引入后会让业务受框架类型和生命周期约束。

现阶段可直接复用的真实 seam：

- `context.Context`：取消、deadline、request ID、语言；
- `pkg/http.Client`：模型网关、流式响应、重试和连接池；
- `pkg/mcp`：MCP transport；
- `zlog` 与 Prometheus middleware：调用链日志和低基数指标；
- cron/cycle：明确 owner 的后台执行。

当业务出现多个模型 provider 或 tool runtime 时，在业务 consumer 侧定义最小 interface。需要跨 HTTP/queue 的 trace 时，优先新增独立 OpenTelemetry adapter module，避免让 core package 直接依赖 SDK。

## 测试位置

单元测试和 package 级集成测试与源码同目录，使用 `package x` 或 `package x_test`。fixture 放 `testdata/`。只有真正跨进程、跨 module 的系统测试才建立独立仓库或明确的 system-test module；当前无需顶层 `tests/`。
