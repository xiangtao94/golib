# mongodb

独立的 MongoDB adapter module，直接使用官方
`go.mongodb.org/mongo-driver/v2`。本模块只拥有连接配置、启动 Ping、连接池和
关闭生命周期；collection、查询、索引和 repository 语义由业务持有。

```go
config := mongodb.DefaultConfig()
config.URI = os.Getenv("MONGODB_URI")
config.Database = "users"
config.AppName = "user-service"

client, err := mongodb.New(ctx, config)
if err != nil {
	return err
}
defer client.Close(shutdownContext)

users := client.Database().Collection("users")
```

安全默认值：

- `mongodb+srv://` 默认接受其 TLS 行为；
- `mongodb://` 必须显式包含 `tls=true` 或 `ssl=true`；
- 禁止关闭证书或主机名验证；
- 只有本地开发可显式设置 `AllowInsecureTransport`；
- 建连、server selection 和启动 Ping 均有 5 秒上限；
- 最大连接池默认 100；
- 初始化错误会清理 driver，错误信息会脱敏 URI 凭证。

模块不会创建业务 collection，不提供通用 DAO，也不为审计新增存储。
