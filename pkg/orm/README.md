# ORM

这个包提供了对MySQL和内存数据库的ORM支持，基于GORM框架。

## 功能特性

- 支持MySQL数据库连接
- 支持SQLite内存数据库
- 统一的接口设计，便于数据库切换
- 事务管理支持
- 分页查询支持
- 日志记录和监控

## 使用示例

### MySQL数据库

```go
import "github.com/xiangtao94/golib/pkg/orm"

// 配置MySQL
mysqlConf := orm.MysqlConf{
    DataBase: "test_db",
    Addr:     "localhost:3306",
    User:     "root",
    Password: "password",
    Charset:  "utf8mb4",
}

// 初始化MySQL客户端
db, err := orm.InitMysqlClient(mysqlConf)
if err != nil {
    panic(err)
}

// 使用事务管理器
tm := orm.NewTransactionManager(ctx, db)
err = tm.ExecuteInTransaction(
    func(tx *gorm.DB) error {
        // 执行数据库操作
        return tx.Create(&user).Error
    },
)
```

### 内存数据库

```go
import "github.com/xiangtao94/golib/pkg/orm"

// 配置内存数据库
memoryConf := orm.MemoryConf{
    DatabaseName: "test_memory_db",
}

// 初始化内存数据库客户端
db, err := orm.InitMemoryClient(memoryConf)
if err != nil {
    panic(err)
}

// 使用事务管理器
tm := orm.NewMemoryTransactionManager(ctx, db)
err = tm.ExecuteInTransaction(
    func(tx *gorm.DB) error {
        // 执行数据库操作
        return tx.Create(&user).Error
    },
)
```

### 数据库切换

为了方便数据库切换，可以创建一个统一的数据库工厂：

```go
type DatabaseType string

const (
    DatabaseTypeMySQL  DatabaseType = "mysql"
    DatabaseTypeMemory DatabaseType = "memory"
)

type DatabaseFactory struct {
    dbType DatabaseType
    db     *gorm.DB
}

func NewDatabaseFactory(dbType DatabaseType) (*DatabaseFactory, error) {
    var db *gorm.DB
    var err error
    
    switch dbType {
    case DatabaseTypeMySQL:
        conf := orm.MysqlConf{
            // MySQL配置
        }
        db, err = orm.InitMysqlClient(conf)
    case DatabaseTypeMemory:
        conf := orm.MemoryConf{
            DatabaseName: "app_db",
        }
        db, err = orm.InitMemoryClient(conf)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", dbType)
    }
    
    if err != nil {
        return nil, err
    }
    
    return &DatabaseFactory{
        dbType: dbType,
        db:     db,
    }, nil
}

func (f *DatabaseFactory) GetDB() *gorm.DB {
    return f.db
}

func (f *DatabaseFactory) NewTransactionManager(ctx *gin.Context) interface{} {
    switch f.dbType {
    case DatabaseTypeMySQL:
        return orm.NewTransactionManager(ctx, f.db)
    case DatabaseTypeMemory:
        return orm.NewMemoryTransactionManager(ctx, f.db)
    default:
        return nil
    }
}
```

## 共用结构

### CrudModel

```go
type CrudModel struct {
    CreatedAt time.Time      `json:"createdAt" gorm:"comment:创建时间"`
    UpdatedAt time.Time      `json:"updatedAt" gorm:"comment:最后更新时间"`
    DeletedAt gorm.DeletedAt `json:"-" gorm:"index;comment:删除时间"`
}
```

### 分页查询

```go
page := &orm.NormalPage{
    No:      1,
    Size:    10,
    OrderBy: "id desc",
}

var users []User
db.Scopes(orm.NormalPaginate(page)).Find(&users)
```

## 注意事项

1. **内存数据库特点**：
   - 数据存储在内存中，应用重启后数据会丢失
   - 适合测试环境或临时数据存储
   - 性能较高，但容量受内存限制

2. **数据库切换**：
   - 两种数据库使用相同的GORM接口
   - 事务管理器接口略有不同，建议使用工厂模式统一管理
   - 配置参数不同，需要根据数据库类型设置相应配置

3. **监控和日志**：
   - 都支持Prometheus监控
   - 使用统一的日志格式
   - 支持请求ID追踪

```go
// 日志示例输出
// {"level":"DEBUG","time":"2024-01-01 12:00:00.123","msg":"mysql","sql":"SELECT * FROM users WHERE age > ? ORDER BY created_at desc LIMIT 10","rows":5,"cost":"2.5ms","requestId":"req-123"}
```

## 配置默认值

如果配置项为空，系统会使用以下默认值：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| MaxIdleConns | 50 | 最大空闲连接数 |
| MaxOpenConns | 50 | 最大打开连接数 |
| ConnMaxIdlTime | 5分钟 | 连接最大空闲时间 |
| ConnMaxLifeTime | 10分钟 | 连接最大生存时间 |
| ConnTimeOut | 3秒 | 连接超时时间 |
| WriteTimeOut | 1200毫秒 | 写超时时间 |
| ReadTimeOut | 1200毫秒 | 读超时时间 |

## 注意事项

- 使用 `CrudModel` 可以获得标准的创建、更新、删除时间字段
- 删除操作默认为软删除，不会物理删除数据
- 连接池配置需要根据实际负载进行调优
- 所有数据库操作都会记录详细日志，便于调试和监控
- 支持时区设置，默认使用 Asia/Shanghai 