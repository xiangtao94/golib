# Render

Gin JSON/SSE 输出 adapter。默认 response：

```json
{
  "code": 200,
  "message": "success",
  "request_id": "...",
  "data": {}
}
```

## 实例级 renderer

```go
renderer := render.NewJSONRenderer(func() render.Render {
	return &BusinessResponse{}
})

renderer.Success(ctx, data)
renderer.Failure(ctx, err)
```

factory 归 `JSONRenderer` 实例所有，不存在 package 全局注册和测试串扰。自定义 response 直接实现 `render.Render`。

`Failure` 识别 `errors.Error` 的业务码与 HTTP status；其他 error 返回系统错误。request ID 来自标准 request context，缺失时生成并写入 response header/body。

SSE 使用 `RenderStream` 和 `RenderStreamFail`。调用方先安装 `middleware.UploadEventStream`，并单独配置 CORS。
