# otel

可选 OpenTelemetry instrumentation module：

- `Gin` 创建入站 HTTP span，并把 `trace_id`、`span_id` 注入 zlog context fields；
- `HTTPTransport` 适配 `pkg/httpclient` 的 `TransportMiddleware`，创建出站 span 并传播 trace context；
- `WithTraceFields` 可用于 job 或其他非 HTTP span 的日志关联。

应用负责创建和关闭 tracer/meter provider，选择 exporter、resource、sampler 与 propagator。本 module 不初始化 SDK，也不读取 OTLP 环境变量。
