# Flow

Flow 提供轻量分层约定，不实现 service locator，也不持有进程级业务状态。

## Controller

```go
type CreateUserController struct {
    users UserService
}

func (controller *CreateUserController) Action(
    ctx context.Context,
    request *CreateUserRequest,
) (any, error) {
    return controller.users.Create(ctx, request)
}

router.POST("/users", flow.Use(func() flow.IController[CreateUserRequest] {
    return &CreateUserController{users: users}
}))
```

factory 每个请求执行一次，因此构造器注入的依赖会完整保留。绑定和 JSON 渲染由 Gin adapter 负责；核心 Controller 只依赖 `context.Context`。

可通过 `flow.WithBinding` 选择无 Content-Type 请求的绑定器，通过 `flow.WithRenderer` 注入实例级 renderer。

## Layer

```go
service := flow.Create(ctx, func() *UserService {
    return &UserService{repository: repository}
})
```

`Create` 不使用反射。factory 创建对象后，Flow 设置标准 context 并调用 `OnCreate`。

## DB registry

```go
registry := flow.NewDBRegistry(primary, map[string]*gorm.DB{
    "analytics": analytics,
})

dao := &UserDAO{Dao: flow.NewDao(registry)}
dao.SetCtx(ctx)
```

Registry 是普通实例，可按应用或测试独立创建。不存在可被其他测试或请求覆盖的全局 DB。未配置 DB 时 CommonDao 返回 `flow.ErrDatabaseNotConfigured`，不会 nil pointer panic。

## API adapter

`Api` 接受全部 2xx 状态，包括 201 和 204；transport error 优先返回，nil response/data 会得到明确错误。
