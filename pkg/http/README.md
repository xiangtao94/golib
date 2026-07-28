# HTTP client

该 package 在 Resty 之上提供一个持有连接池生命周期的 `Client`。配置与运行时对象分离；它不依赖 Gin。

## 创建

```go
config := httpclient.DefaultClientConfig()
config.Service = "users"
config.Domain = "https://users.example.com"
config.RetryCount = 0

client, err := httpclient.NewClient(config)
if err != nil {
	return err
}
defer client.Close()
```

`DefaultClientConfig` 提供 5 秒请求/连接 timeout 和 3 次重试。直接使用 `ClientConfig{}` 时，零值是显式语义：`RetryCount == 0` 不重试，body 日志关闭，未设置 timeout 时由调用 context 控制。

`NewClient` 会校验重试数和多 domain 负载均衡配置。初始化失败返回 error，不延迟到第一次请求。

## 请求

```go
result, err := client.Post(ctx, httpclient.RequestOptions{
	Path:   "/v1/users",
	Encode: httpclient.EncodeJson,
	RequestBody: map[string]any{
		"name": "Alice",
	},
	Headers: map[string]string{
		"Authorization": "Bearer ...",
	},
	Timeout: 2 * time.Second,
})
```

支持 `Get`、`Head`、`Post`、`Put`、`Patch`、`Delete`，以及 `GetStream`、`PostStream`。`EncodeJson`、`EncodeForm`、`EncodeRaw`、`EncodeRawByte` 和 `EncodeFile` 控制 body 编码。

所有方法要求非 nil `context.Context`。client 会保证 request ID 存在并通过 `X-Request-ID` 传给下游；取消和 deadline 贯穿普通与流式请求。

## 多地址与重试

```go
client, err := httpclient.NewClient(httpclient.ClientConfig{
	Service:    "model-gateway",
	Domains:    []string{"https://model-a.example.com", "https://model-b.example.com"},
	RetryCount: 2,
	RetryCondition: func(response *resty.Response, err error) bool {
		return err != nil || response.StatusCode() >= 500
	},
})
```

`Domains` 默认使用 round-robin，也可注入 Resty `LoadBalancer`。重试可能重复副作用；业务必须只为幂等请求或具备幂等键的写请求启用。

## 流式响应

```go
_, err := client.PostStream(ctx, options, func(line []byte) error {
	return consume(line)
})
```

stream handler 不能为空。响应按行交付，单行最大 100 MiB；非 2xx 响应返回 error。若业务需要原始 chunk、SSE event 解析或更小上限，应在业务 module 定义专用 adapter。

## 日志与资源

`MaxReqBodyLen` 和 `MaxRespBodyLen` 只有为正数时才记录有界 body；零值和负数都不记录。`TraceEnabled` 显式开启 DNS、连接、TLS、首字节等 trace。

一个 `Client` 应长期复用，并在 owner 退出时调用 `Close`。`Close` 幂等，会关闭空闲连接和 Resty 资源。
