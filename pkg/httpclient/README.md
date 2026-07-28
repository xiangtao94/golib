# httpclient

`httpclient` 只负责创建带安全默认值和基础观测能力的具体
`*resty.Client`。请求构造、类型化响应、上传、下载和 SSE 直接使用 Resty
API，不再维护第二套 HTTP DSL。

```go
config := httpclient.DefaultConfig()
config.Service = "billing"
config.BaseURL = "https://billing.internal"

client, err := httpclient.New(config)
if err != nil {
	return err
}
defer client.Close()

var invoice Invoice
response, err := client.R().
	SetContext(ctx).
	SetResult(&invoice).
	SetBody(CreateInvoiceRequest{Amount: 100}).
	Post("/invoices")
```

默认行为：

- 单次请求和连接建立均有 5 秒上限；
- `RetryCount == 0`，不默认重试；
- HTTP 明文地址被拒绝，只有本地开发可显式设置 `AllowHTTP`；
- 日志只记录 method、去 query/凭证后的 URL、状态、attempt 和耗时，不记录
  header 或 body；
- 自动传播 `X-Request-ID`；
- Resty 默认只重试幂等方法。业务显式开启非幂等重试时，必须同时提供
  `Idempotency-Key`；
- transport middleware 用于 OpenTelemetry 等真实横切能力。

多实例地址使用 `BaseURLs`，内部交给 Resty 的 round-robin load balancer。
`BaseURL` 与 `BaseURLs` 不能同时设置。调用方是 client 的资源 owner，必须调用
`Close`。
