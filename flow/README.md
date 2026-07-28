# Flow

Flow 是根 module 内的 HTTP adapter 与基础数据访问工具，不是 service locator，也不持有请求级或进程级业务状态。

## Controller seam

业务直接实现 `Controller[T]`：

```go
type CreateUserController struct {
	users *UserApplication
}

func (controller *CreateUserController) Action(
	ctx context.Context,
	request *CreateUserRequest,
) (any, error) {
	return controller.users.Create(ctx, request)
}

router.POST("/users", flow.Use(func() flow.Controller[CreateUserRequest] {
	return &CreateUserController{users: users}
}))
```

factory 每个请求执行一次。Gin adapter 负责绑定、request ID 和渲染；业务只接收标准 `context.Context`。可通过 `WithBinding` 和 `WithRenderer` 替换 adapter 行为。

## DB registry 与 DAO

```go
registry := flow.NewDBRegistry(primary, map[string]*gorm.DB{
	"analytics": analytics,
})

type UserDAO struct {
	flow.Dao
}

users := &UserDAO{Dao: flow.NewDao(registry)}
db := users.GetDB(ctx)
analyticsDB := users.GetDBByName(ctx, "analytics")
```

`DBRegistry` 在构造后只读，按应用或测试实例注入，不存在包级默认 DB。`CommonDao[T]` 的所有操作显式接收 context；未配置 DB 时返回 `ErrDatabaseNotConfigured`。

## 外部 HTTP

```go
client, err := httpclient.NewClient(httpclient.ClientConfig{
	Domain: "https://users.example.com",
})
if err != nil {
	return err
}
defer client.Close()

usersAPI := flow.Api{Client: client}
response, err := usersAPI.ApiGet(ctx, "/v1/users/1", nil)
```

`Api` 是具体类型，不再通过同形的 `IApi` 暴露浅 interface。全部方法显式接收 context，并接受所有 2xx HTTP 状态。
