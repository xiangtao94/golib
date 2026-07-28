# 多语言错误

`errors.Error` 是可复制的业务错误值，包含业务码、HTTP status 和多语言消息。核心错误处理只依赖标准 context。

```go
var ErrUserNotFound = errors.NewError(1001, map[string]string{
	"zh": "用户不存在",
	"en": "User not found",
}).WithHTTPStatus(http.StatusNotFound)

ctx := env.WithLanguage(context.Background(), env.LanguageEnglish)
message := ErrUserNotFound.GetMessage(ctx)
```

`Sprintf` 返回格式化后的副本，不修改共享错误值。`WithHTTPStatus` 只接受 4xx/5xx；非法值归一为 500。

预置错误包括 `ErrorParamInvalid`、`ErrorSystemError`、`ErrorUserNotLogin`、`ErrorInvalidRequest` 和 `ErrorDefault`。HTTP 输出由 `render.JSONRenderer` 负责，业务逻辑不应依赖 Gin。
