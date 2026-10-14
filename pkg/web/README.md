# Web

`web.Handle` 将 `func(*gin.Context, *Request) (any, error)` 适配成 Gin handler，统一参数绑定、request ID、错误日志和 JSON 渲染。支持普通函数、闭包及方法值，不要求 Controller 接口或空结构体。

```go
func Login(ctx *gin.Context, request *LoginRequest) (any, error) {
    return auth.Login(ctx.Request.Context(), request)
}
router.POST("/auth/login", web.Handle(Login))
```

默认按 Gin 的请求方法和 Content-Type 选择 binding；`WithBinding(binding.JSON)` 显式强制指定 binder。`WithRenderer` 配置响应结构，`WithSuccessStatus` 配置成功状态码（例如 202）。

所有 JSON 接口（包括 multipart 上传）使用同一 Handle 签名。HTML、媒体流使用普通 Gin handler，JSON 成功和失败响应复用 `render.JSONRenderer`。不再支持 `Controller[T]`、`ControllerOption` 或 `ShouldRender`：调用方将 `web.Handle(controller)` 改为 `web.Handle(controller.Action)`，或直接传入函数；入口的第一个参数统一改为 `*gin.Context`，调用 Service 时传入 `c.Request.Context()`；选项类型改为 `web.Option`。

失败日志使用请求的标准 context，包含 `requestId`、`path`、展开的私有错误诊断 `error` 和日志现场 `stacktrace`。公共错误响应保持不变；内部 cause 和调用栈只进入服务端日志。
