# Breaking migration

本轮不保留兼容层。调用方应一次性迁移，并发布新的 major version。

## Flow

- `IController[T]` 改为 `Controller[T]`。
- `Action(*T)` 改为 `Action(context.Context, *T)`。
- `flow.Use(controller)` 改为 `flow.Use(func() flow.Controller[T] { return controller })`。
- 删除 `ILayer`、`IService`、`IData`、`IApi`、`IDao`、`Create`、`SetCtx`、`Reset`。
- `Api` 和所有 DAO 方法显式接收 context。
- 删除全局 DB setter；构造并注入 `DBRegistry`。

## HTTP client

- `ClientConf` 改为配置 `ClientConfig` 与运行时 `Client`。
- 使用 `NewClient(config)`，处理初始化 error，并由 owner 调用 `Close`。
- `RetryCount == 0` 明确表示不重试。
- nil context 和 nil stream handler 返回明确 error。

## env 与错误

- `env` 成为独立 module。
- 删除 `LoadConfByEnv`、Gin mode/Docker 推断、本地 IP 与日志目录 helper。
- `APP_NAME` 环境变量替代旧的 `XT_APP_NAME`。
- 请求级语言改为 `env.WithLanguage` / `LanguageFromContext`。
- `errors.Error.GetMessage` 接收标准 context。

## zlog 与 middleware

- `zlog` 成为独立且不依赖 Gin/Viper 的 module。
- `InitLog(LogConfig)` 返回 `(*zap.SugaredLogger, error)`；`CloseLogger` 返回 error。
- `Buffer.Switch string` 改为 `Buffer.Enabled bool`。
- request ID 只存标准 context；HTTP/Gin 转换由 `middleware.RequestID` 负责。
- access 自定义字段改用 `middleware.AddAccessFields`。

## Job 与 render

- cron/cycle job 改为 `Run(context.Context) error`。
- `Start` 接收 parent context，`Stop` 接收 shutdown context 并返回 error。
- 删除全局 render 注册，改为注入 `render.NewJSONRenderer(factory)`。
