# Implementation Plan: Production Architecture Hardening

## Overview

按已确认的风险顺序把当前 Gin 强耦合工具集合重构为安全默认、可组合、可测试的 Go 基础库。当前没有下游服务，因此允许破坏公共 API，但每个阶段必须保持仓库可编译，并用回归测试证明行为。

## Architecture Decisions

- 调试、请求正文日志、宽松 CORS 等能力全部改为显式启用。
- 核心层和外部客户端统一接收 `context.Context`，Gin 只存在于 HTTP adapter。
- Controller 使用显式 factory 创建，禁止通过反射创建零值对象。
- HTTP Server 由调用方传入生命周期 context，基础库不接管 OS signal。
- 数据库和 renderer 通过实例注入，不使用可变包级业务状态。
- 不为审计单独增加任何业务存储。

## Task List

### Phase 1: Security Blockers

- [x] pprof 改为显式注册，默认路由不可见。
- [x] 分页排序改为字段白名单和结构化方向。
- [x] Timeout middleware 改为协作式 context 取消，不并发操作 Gin writer。
- [x] Access log 使用有界前缀捕获，默认不记录正文，跳过路径不分配捕获缓冲。
- [x] Recovery 真正注册且不修改共享 logger。

### Checkpoint: Security

- [x] 针对每个阻断问题的回归测试通过。
- [x] `go test ./pkg/middleware ./pkg/orm .` 通过。

### Phase 2: Lifecycle and Context

- [x] HTTP Server 提供安全超时、`Run(ctx)` 和确定性 shutdown。
- [x] HTTP、ES、Milvus、MinIO、ORM、Redis、job API 统一使用 `context.Context`。
- [x] 调度器停止时等待执行中的任务并正确传播取消。

### Checkpoint: Lifecycle

- [x] 生命周期和 context 传播测试通过。
- [x] `go test ./... -run '^$'` 编译门禁通过（全量运行测试留在最终门禁）。

### Phase 3: Public Contracts and State

- [x] Controller 改为显式 factory，删除反射构造。
- [x] Flow DB registry 实例化，删除全局 DB client。
- [x] Renderer factory 并发安全且支持实例注入。
- [x] JSON 错误使用正确 HTTP status；客户端接受所有 2xx 并安全处理 nil/error。

### Phase 4: Middleware and External Boundaries

- [x] Rate limiter 按 TTL 回收且代理地址策略可配置。
- [x] Prometheus 使用路由模板和有界 status class labels。
- [x] CORS 使用明确 allowlist；Gzip 正确处理 Vary、Flush、SSE。
- [x] MySQL、Redis、MinIO 支持安全 TLS 配置。

### Phase 5: Storage and Jobs

- [x] Milvus 已存在 collection 时验证 schema；搜索 ANN 参数显式化。
- [x] Cron/Cycle 去除 Gin 依赖并修复并发生命周期。
- [x] Cache janitor 可幂等停止，序列化错误完整返回。

### Phase 6: Module Boundaries and Final Gate

- [x] 可选基础设施 adapter 拆分为独立 Go modules。
- [x] README、示例和迁移说明与新契约一致。
- [x] 全部 module 的测试、race、vet、漏洞扫描通过。

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| 公共接口大范围变化 | 编译期破坏 | 当前无消费者；同步更新示例和编译测试 |
| 多模块拆分导致循环依赖 | 构建失败 | 先去除 root 对 adapter 的反向引用，再拆模块 |
| Timeout 行为无法强制终止副作用 | 业务继续执行 | 采用 context 协作取消并明确契约，禁止并发 writer |
| 日志脱敏无法猜测业务字段 | PII 泄露 | 默认关闭正文，显式启用时要求 caller 提供 redactor |

## Open Questions

- 无阻塞问题；按“当前无服务使用”执行破坏性重构。
