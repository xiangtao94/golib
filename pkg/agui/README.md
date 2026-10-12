# AG-UI gateway

`pkg/agui` 是独立可选 module，用于多个 Gin 业务服务代理外部 Agent 服务的 AG-UI 1.0 HTTP/SSE 接口。它接收一次 `POST RunAgentInput`，将 JSON 请求发往固定配置的 Agent 地址，逐帧转发 `text/event-stream`，并保持事件原始 JSON 和顺序。它不实现 Agent、MCP、会话存储或模型调用。

```go
app, err := golib.NewApp("agent-gateway", golib.WithAccessLog())
if err != nil {
	return err
}
defer app.Stop(context.Background()) // 调用 Run 前的启动失败也会回收资源。

gateway, err := agui.New(agui.Config{
	Endpoint: "https://agent.internal/run",
	Headers: func(ctx context.Context) (http.Header, error) {
		// 从服务端可信 context 取身份，再签发给 Agent 服务的凭证。
		return http.Header{"Authorization": {"Bearer " + serviceToken(ctx)}}, nil
	},
})
if err != nil {
	return err
}

if err := app.AddStop("agui-gateway", func(context.Context) error {
	gateway.Close()
	return nil
}); err != nil {
	gateway.Close()
	return err
}
group := app.Group("/api", authenticateAndAuthorize())
gateway.Register(group, "/agent/run")

server := agui.NewHTTPServer(app.Engine, golib.DefaultHTTPServerConfig(8080))
return app.RunServer(appContext, server)
```

`Register` 只安装 AG-UI 的 `POST` 路由，流开始时自动设置 SSE header 并逐帧 flush。`NewHTTPServer` 包装根库的 server，保留原始 response writer，使 AG-UI 请求能单独清除写入期限；普通路由继续使用配置中的 `WriteTimeout`。自行创建 `http.Server` 时，将 Gin engine 包在 `agui.Wrap(engine)` 后再传给 server。没有包装时，AG-UI 路由在调用上游前返回 500。

默认创建的 HTTP transport 由 `Gateway.Close` 关闭空闲连接；注入的 `Client` 由调用方管理。示例将 gateway 登记进 `App` 的生命周期，确保 HTTP server 退出后、日志关闭前清理连接。

默认限制：请求体 4 MiB、单个 SSE 帧 8 MiB、每个 gateway 实例最多同时 128 个流、一次 run 最长 30 分钟。另有默认 256 MiB 的在途载荷准入预算；实际并发上限取 `MaxConcurrent` 与 `MaxInFlightPayloadBytes / (MaxRequestBytes + MaxEventBytes)` 的较小值，默认配置为 21 个流。预算根据配置上限预留，不等于进程堆内存的精确上限。均可在 `Config` 中设置正数覆盖。默认上游 client 限制响应头等待 30 秒、验证 TLS、不跟随重定向；传入自定义 `Client` 时不能设置 `Client.Timeout`，因为它会截断长流。HTTP 明文地址须显式设 `AllowHTTP`。不自动重试 run，也不支持 `Last-Event-ID` 续流。

上游在开流前拒绝请求时，gateway 返回对应 HTTP 状态；开流后如果目标 run 未收到终止事件就被上游截断，会发出 `RUN_ERROR`，错误码为 `UPSTREAM_STREAM_ERROR`。前端断开时，请求 context 会取消上游请求。客户端提供的 `threadId`、`tools`、`context` 和 `forwardedProps` 都不能当作可信权限；业务 middleware 与 Agent 服务须基于服务端身份重新检查范围。浏览器的 Authorization/Cookie 不会自动转发，跨域策略由业务的 CORS 配置决定。

模块只代理 AG-UI HTTP/SSE。若上游或前端协商 Protobuf，或需要 Agent 业务语义变换，由业务服务单独实现。不要用 `render.RenderStreamFail` 包裹 AG-UI 事件流。
