# Middleware

## 推荐顺序

```go
cors, err := middleware.NewCORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowCredentials: true,
})
if err != nil {
    return err
}

engine.Use(middleware.Recovery(nil, nil))
engine.Use(middleware.RequestID())
engine.Use(cors)
engine.Use(middleware.RateLimitMiddleware(middleware.RateLimiterConfig{
    Rate:       100,
    Burst:      200,
    TTL:        10 * time.Minute,
    MaxEntries: 50_000,
}))
engine.Use(middleware.GzipMiddleware())
```

Access log 默认不捕获 body。正文日志只有在 `MaxReqBodyLen`/`MaxRespBodyLen` 为正且配置 `BodySanitizer` 时启用。Authorization、Cookie、Proxy-Authorization、X-Api-Key 永不记录。

Prometheus：

```go
metrics, err := middleware.NewMetrics(middleware.MetricsConfig{
    AppName: "user-service",
    Collectors: []prometheus.Collector{dbCollector},
})
if err != nil {
    return err
}
middleware.RegisterMetrics(engine, metrics)
```

指标使用 Gin route template、method、app name 和 status class。不要把用户 ID、query 或原始 URL 加入 label。

SSE route 使用 `UploadEventStream` 设置 transport header；跨域策略统一由 `NewCORS` 管理。

`golib.Bootstrap` 会自动安装 `RequestID`，手动组装 middleware 时才需显式注册。业务日志从 `ctx.Request.Context()` 读取 request ID；access log 自定义字段使用 `middleware.AddAccessFields(ctx, fields...)`。
