# errors

`Error` 是不可变的公共服务错误，稳定契约包含：

- machine-readable `code` 和 `reason`；
- HTTP status；
- 对外 message；
- retryable；
- 可选 details；
- 只供日志和 `errors.Is` / `errors.As` 使用的私有 cause。

`New(code, message, httpStatus...)` 的 HTTP 状态码可省略，默认 500；显式状态码必须在 400–599，否则回退到 500。最多传入一个状态码，传入多个会 panic。现有三参数调用保持不变；如果将 `New` 保存到固定签名的函数变量中，需要将变量签名同步改为 `func(string, string, ...int) *errors.Error`。

```go
var ErrMissingMySQL = errors.New("MISSING_MYSQL", "数据库配置缺失") // 默认 500
var ErrUserNotFound = errors.New(
	"USER_NOT_FOUND",
	"user not found",
	http.StatusNotFound,
).WithReason("NOT_FOUND")

return ErrUserNotFound.Wrap(databaseErr)
```

`Error()` 不包含 cause。未知错误经 `From` 转为安全的 `INTERNAL`，不会把内部错误文本返回客户端。业务错误码和本地化消息由拥有该业务的服务定义。

`Wrap` 捕获包装位置的调用栈。`Diagnostic(err)` 仅供服务端日志使用，展开普通包装及 `errors.Join` 的 cause 和包装栈，最多遍历 64 个错误节点。它不改变 `Error()`、响应字段或 `errors.Is` / `errors.As`；不要将诊断文本返回客户端。包装栈定位首次经过 `Wrap` 的位置，无法还原此前已返回的底层调用。
