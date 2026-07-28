# Render

Gin JSON/SSE 输出 adapter。默认 response：

```json
{
  "code": "OK",
  "reason": "OK",
  "message": "success",
  "request_id": "...",
  "retryable": false,
  "data": {}
}
```

## 实例级 renderer

```go
renderer := render.NewJSONRenderer(func(response render.Response) any {
	return BusinessResponse{
		Code:    response.Code,
		Message: response.Message,
		Data:    response.Data,
	}
})

renderer.Success(ctx, data)
renderer.Failure(ctx, err)
```

factory 归 `JSONRenderer` 实例所有，不存在 package 全局注册和测试串扰。factory 只在 HTTP 边界映射稳定的 `render.Response`，不改变业务错误。

`Failure` 识别 `errors.Error` 的稳定 code、reason、HTTP status、retryable 和 details；其他 error 返回安全的 `INTERNAL`。request ID 来自标准 request context，缺失时生成并写入 response header/body。

SSE 使用 `RenderStream` 和 `RenderStreamFail`。调用方先安装 `middleware.UploadEventStream`，并单独配置 CORS。
