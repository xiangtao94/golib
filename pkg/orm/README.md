# ORM

独立 MySQL/GORM adapter。该 module 不提供内存数据库、备份或业务存储扩展。

## 连接

```go
db, err := orm.InitMysqlClient(orm.MysqlConf{
	DataBase:      "users",
	Addr:          "mysql.example.com:3306",
	User:          "users",
	Password:      password,
	Charset:       "utf8mb4",
	TLSConfigName: "true",
})
if err != nil {
	return err
}
```

默认要求验证 TLS。仅本地开发确需明文时，必须显式设置 `AllowInsecureTransport: true`。零值连接池与 timeout 会使用 package 默认值。

## 安全分页

```go
scope := orm.NormalPaginate(page, map[string]string{
	"createdAt": "created_at",
	"name":      "name",
})
err := db.Scopes(scope).Find(&users).Error
```

调用方必须提供公开排序名到可信数据库列名的 allowlist，避免把用户输入直接拼入 `ORDER BY`。page size 上限为 100。

## 事务与指标

```go
manager := orm.NewTransactionManager(ctx, db)
err := manager.ExecuteInTransaction(
	func(tx *gorm.DB) error { return tx.Create(&user).Error },
	func(tx *gorm.DB) error { return tx.Create(&auditEvent).Error },
)
```

业务数据和审计数据仍由业务选择现有存储；基础库不会为了审计创建额外业务存储。

`NewMySQLPrometheusCollector` 返回标准 collector，由根 module 的 metrics middleware 注册。
