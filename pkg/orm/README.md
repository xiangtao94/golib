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

## 事务与指标

```go
err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
	if err := tx.Create(&user).Error; err != nil {
		return err
	}
	return tx.Create(&existingBusinessRecord).Error
})
```

分页、排序、模型字段和事务内操作都属于业务 repository。adapter 不替业务定义查询策略，也不会为了审计创建额外业务存储。

`NewMySQLPrometheusCollector` 返回标准 collector，由根 module 的 metrics middleware 注册。
