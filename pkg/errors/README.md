# errors

`Error` 是不可变的公共服务错误，稳定契约包含：

- machine-readable `code` 和 `reason`；
- HTTP status；
- 对外 message；
- retryable；
- 可选 details；
- 只供日志和 `errors.Is` / `errors.As` 使用的私有 cause。

```go
var ErrUserNotFound = errors.New(
	"USER_NOT_FOUND",
	"user not found",
	http.StatusNotFound,
).WithReason("NOT_FOUND")

return ErrUserNotFound.Wrap(databaseErr)
```

`Error()` 不包含 cause。未知错误经 `From` 转为安全的 `INTERNAL`，不会把内部错误文本返回客户端。业务错误码和本地化消息由拥有该业务的服务定义。
