# Web

`web` 把业务 Controller 适配成 Gin handler，统一负责参数绑定、request ID、错误映射和 JSON 渲染。业务 Controller 本身只依赖标准 `context.Context`。

```go
type CreateUserController struct {
	users *application.CreateUser
}

func (controller *CreateUserController) Action(
	ctx context.Context,
	request *CreateUserRequest,
) (any, error) {
	return controller.users.Execute(ctx, request)
}

controller := &CreateUserController{users: createUser}
router.POST("/users", web.Handle[CreateUserRequest](controller))
```

Controller 在应用组装阶段构造并长期复用，只持有构造期依赖。请求状态通过 `Action` 参数传入，不应保存在 Controller 字段中。

无 `Content-Type` 时默认使用 form binding；可通过 `WithBinding` 调整。自定义 response shape 通过实例级 `WithRenderer` 注入。
