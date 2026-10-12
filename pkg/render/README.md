# Render

Gin JSON/SSE 输出 adapter。默认 response：

```json
{
  "code": 200,
  "data": {},
  "reason": "OK",
  "message": "success",
  "request_id": "..."
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

默认 JSON 的 `code` 与 HTTP 状态码一致，`data` 始终存在。失败时 `data` 为 `null`，稳定的业务错误标识位于 `error_code`；自定义 factory 仍可读取 `render.Response.Code`。异步受理可调用 `renderer.SuccessWithStatus(ctx, http.StatusAccepted, data)`，或在 `web.Handle` 中设置 `web.WithSuccessStatus(http.StatusAccepted)`。

例如校验失败响应为 `{"code":400,"data":null,"error_code":"INVALID_ARGUMENT","reason":"INVALID_ARGUMENT","message":"...","request_id":"..."}`。

`Failure` 识别 `errors.Error` 的稳定 code、reason、HTTP status、retryable 和 details；其他 error 返回安全的 `INTERNAL`。request ID 来自标准 request context，缺失时生成并写入 response header/body。

SSE 使用 `RenderStream` 和 `RenderStreamFail`。调用方先安装 `middleware.UploadEventStream`，并单独配置 CORS。
