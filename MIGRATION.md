# Breaking migration

本轮不保留兼容层。调用方应一次性迁移，并发布新的 major version。

## Web、下游调用与数据库

- 删除 `flow` package。
- `flow.Controller[T]` 改为 `web.Controller[T]`，`Action` 保持接收 `context.Context`。
- `flow.Use(factory)` 改为 `web.Handle(controller)`；Controller 在组装阶段直接注入，不再每请求构造。
- 删除固定 `ApiRes/Api` 业务协议封装；业务使用 Core HTTP client 实现类型化 client。
- 删除 `Dao/CommonDao/DBRegistry`；业务 repository 直接使用可选 ORM adapter，并拥有查询与错误语义。

## HTTP client

- `ClientConf` 改为配置 `ClientConfig` 与运行时 `Client`。
- 使用 `NewClient(config)`，处理初始化 error，并由 owner 调用 `Close`。
- `RetryCount == 0` 明确表示不重试。
- nil context 和 nil stream handler 返回明确 error。

## env 与错误

- `env` 归入统一 Core module，但保持独立 package 和框架无关实现。
- 删除 `LoadConfByEnv`、Gin mode/Docker 推断、本地 IP 与日志目录 helper。
- `APP_NAME` 环境变量替代旧的 `XT_APP_NAME`。
- 请求级语言改为 `env.WithLanguage` / `LanguageFromContext`。
- `errors.Error.GetMessage` 接收标准 context。

## zlog 与 middleware

- `zlog` 归入统一 Core module，package 本身仍不依赖 Gin/Viper。
- `InitLog(LogConfig)` 返回 `(*zap.SugaredLogger, error)`；`CloseLogger` 返回 error。
- `Buffer.Switch string` 改为 `Buffer.Enabled bool`。
- request ID 只存标准 context；HTTP/Gin 转换由 `middleware.RequestID` 负责。
- access 自定义字段改用 `middleware.AddAccessFields`。

## Job 与 render

- cron/cycle job 改为 `Run(context.Context) error`。
- `Start` 接收 parent context，`Stop` 接收 shutdown context 并返回 error。
- 删除全局 render 注册，改为注入 `render.NewJSONRenderer(factory)`。
