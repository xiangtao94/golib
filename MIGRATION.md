# Breaking migration

本轮不保留兼容层。调用方应一次性迁移，并发布新的 major version。

## Web、下游调用与数据库

- 删除 `flow` package。
- `flow.Controller[T]` 改为 `web.Controller[T]`，`Action` 保持接收 `context.Context`。
- `flow.Use(factory)` 改为 `web.Handle(controller)`；Controller 在组装阶段直接注入，不再每请求构造。
- 删除固定 `ApiRes/Api` 业务协议封装；业务使用 Core HTTP client 实现类型化 client。
- 删除 `Dao/CommonDao/DBRegistry`；业务 repository 直接使用可选 ORM adapter，并拥有查询与错误语义。

## HTTP client

- 删除 `pkg/http` 的 `Client`、`RequestOptions`、`Result`、自定义 verb 和逐行
  stream API，不保留兼容别名。
- import 改为 `pkg/httpclient`，使用 `httpclient.New(Config)` 获取具体的
  `*resty.Client`，并由 owner 调用 `Close`。
- 请求参数、类型化响应、文件和 SSE 直接使用 Resty API。
- `Domain` 改为 `BaseURL`，`Domains` 改为 `BaseURLs`；两者不能同时设置。
- `DefaultConfig` 提供有界 timeout，默认不重试且要求 HTTPS。
- Resty 仍默认只重试幂等方法；显式开启非幂等重试时必须设置
  `Idempotency-Key`。
- retry condition 直接使用 `resty.RetryConditionFunc`；OpenTelemetry 等横切
  能力继续使用 `TransportMiddleware`。

## env 与错误

- `env` 归入统一 Core module，但保持独立 package 和框架无关实现。
- 删除 `env.LoadConf*` 与 `NewViperInstance`，改用 `config.Load[T]`；声明配置文件后默认必须存在。
- 删除 `LoadConfByEnv`、Gin mode/Docker 推断、本地 IP 与日志目录 helper。
- `APP_NAME` 环境变量替代旧的 `XT_APP_NAME`。
- 请求级语言改为 `env.WithLanguage` / `LanguageFromContext`。
- `errors.Error` 改为不可变字符串契约；删除进程级多语言 map、整数 code、`GetMessage` 和 `Sprintf`。
- 业务通过 `errors.New(...).WithReason(...).Wrap(cause)` 定义公共错误；未知错误统一映射为 `INTERNAL`。

## zlog 与 middleware

- `zlog` 归入统一 Core module，package 本身仍不依赖 Gin/Viper。
- `InitLog(LogConfig)` 返回 `(*zap.SugaredLogger, error)`；`CloseLogger` 返回 error。
- `Buffer.Switch string` 改为 `Buffer.Enabled bool`。
- request ID 只存标准 context；HTTP/Gin 转换由 `middleware.RequestID` 负责。
- access 自定义字段改用 `middleware.AddAccessFields`。

## Job 与 render

- cron/cycle job 改为 `Run(context.Context) error`。
- `Start` 接收 parent context，`Stop` 接收 shutdown context 并返回 error。
- 删除 `InitCrontab` 与 `InitCycle` 薄别名，直接使用 `New` 后由 owner 启动。
- lifecycle API 不再把 nil context 替换成 `context.Background()`。
- 删除全局 render 注册，改为注入 `render.NewJSONRenderer(factory)`。
- render factory 改为 `func(render.Response) any`，默认错误响应包含稳定 code/reason/request ID/retryable/details。
- `TimeoutMiddleware` 增加 renderer 参数；传 nil 时使用标准 response contract。

## ORM 与 Redis

- ORM adapter 删除 `CrudModel`、分页/排序类型和 `TransactionManager`；模型、查询和事务操作由业务 repository 持有。
- Redis adapter 删除秒数常量、`GetKeyPrefix` 与 `Clear`；使用 `time.Duration`、显式业务 key namespace 和标准 `Close`。

## MongoDB

- 新增独立 `pkg/mongodb` module，使用官方 MongoDB Go Driver v2。
- `mongodb.New(ctx, Config)` 在返回前完成 primary Ping；调用方同时获得
  `Driver()` 与选定的 `Database()`，并负责 `Close(ctx)`。
- 默认要求 TLS、验证证书与主机名，并限制连接、server selection、Ping
  timeout 和最大连接池。
- collection、索引、查询和 repository 均由业务持有；本模块不新增审计或
  其他业务存储。

## Lifecycle、health 与 telemetry

- 多资源启动/回滚/反向关闭使用 `pkg/lifecycle`；OS signal 仍由应用持有。
- liveness/readiness 使用 `pkg/health.Gate`，新实例默认 not ready。
- OpenTelemetry instrumentation 位于独立 `pkg/otel` module；SDK、exporter、sampler 和 shutdown 由应用持有。

## 对象存储

- 删除 `pkg/oss` MinIO adapter，改用独立模块 `pkg/s3`。
- 删除 `MinioConf`、`MinioClient` 和所有 MinIO SDK 返回类型，不提供兼容别名。
- 使用 `s3.Config` 与 `s3.NewClient`；AWS 可使用默认凭证链，其他 S3-compatible
  服务配置自己的 endpoint、region、寻址风格和凭证。
- 上传、对象信息、列举和复制统一返回 `s3` 模块自有类型。
- `Client.Close` 由创建客户端的基础设施组装层调用。
