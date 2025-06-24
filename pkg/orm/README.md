# ORM

这个包提供了对MySQL和内存数据库的ORM支持，基于GORM框架。

## 功能特性

- 支持MySQL数据库连接
- 支持SQLite内存数据库（多种持久化模式）
- 统一的接口设计，便于数据库切换
- 事务管理支持
- 分页查询支持
- 日志记录和监控
- 数据持久化功能（备份/恢复）

## 内存数据库持久化模式

### 1. 纯内存模式 (PureMemo)
数据完全存储在内存中，应用重启后数据丢失。适合临时计算和测试。

```go
conf := orm.MemoryConf{
    DatabaseName:    "pure_memory_db",
    PersistenceMode: orm.PureMemo,
}

memDB, err := orm.InitMemoryDB(conf)
if err != nil {
    panic(err)
}
defer memDB.Close()

db := memDB.GetDB()
```

### 2. 文件模式 (FileMode)
数据直接存储在磁盘文件中，重启后数据自动恢复。

```go
conf := orm.MemoryConf{
    DatabaseName:    "file_db",
    PersistenceMode: orm.FileMode,
    FilePath:        "./data/app.db",
}

memDB, err := orm.InitMemoryDB(conf)
if err != nil {
    panic(err)
}
defer memDB.Close()

db := memDB.GetDB()
```

### 3. 内存+备份模式 (MemoryWithBackup)
数据在内存中运行，支持定期备份到磁盘和启动时从备份恢复。

```go
conf := orm.MemoryConf{
    DatabaseName:     "memory_backup_db",
    PersistenceMode:  orm.MemoryWithBackup,
    FilePath:         "./backup/app_backup.db",
    BackupInterval:   5 * time.Minute,  // 5分钟备份一次
    AutoBackup:       true,             // 启用自动备份
    BackupOnShutdown: true,             // 关闭时备份
}

memDB, err := orm.InitMemoryDB(conf)
if err != nil {
    panic(err)
}
defer memDB.Close()

db := memDB.GetDB()

// 手动备份
err = memDB.BackupToFile("./manual_backup.db")
if err != nil {
    log.Printf("Manual backup failed: %v", err)
}

// 从备份恢复
err = memDB.RestoreFromFile("./restore_from.db")
if err != nil {
    log.Printf("Restore failed: %v", err)
}
```

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

### 内存数据库（兼容模式）

```go
import "github.com/xiangtao94/golib/pkg/orm"

// 配置内存数据库（向后兼容）
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

### 内存数据库（完整功能）

```go
import "github.com/xiangtao94/golib/pkg/orm"

// 配置内存数据库
memoryConf := orm.MemoryConf{
    DatabaseName:     "test_memory_db",
    PersistenceMode:  orm.MemoryWithBackup,
    FilePath:         "./data/backup.db",
    BackupInterval:   10 * time.Minute,
    AutoBackup:       true,
    BackupOnShutdown: true,
}

// 初始化内存数据库
memDB, err := orm.InitMemoryDB(memoryConf)
if err != nil {
    panic(err)
}
defer memDB.Close()

// 获取数据库连接
db := memDB.GetDB()

// 使用事务管理器
tm := orm.NewMemoryTransactionManager(ctx, db)
err = tm.ExecuteInTransaction(
    func(tx *gorm.DB) error {
        // 执行数据库操作
        return tx.Create(&user).Error
    },
)

// 手动备份
err = memDB.BackupToFile("./manual_backup.db")
if err != nil {
    log.Printf("Backup failed: %v", err)
}

// 控制自动备份
memDB.StopAutoBackup()  // 停止自动备份
memDB.StartAutoBackup() // 重新启动自动备份
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
    memDB  *orm.MemoryDB // 用于内存数据库的完整功能
}

func NewDatabaseFactory(dbType DatabaseType) (*DatabaseFactory, error) {
    var db *gorm.DB
    var memDB *orm.MemoryDB
    var err error
    
    switch dbType {
    case DatabaseTypeMySQL:
        conf := orm.MysqlConf{
            // MySQL配置
        }
        db, err = orm.InitMysqlClient(conf)
    case DatabaseTypeMemory:
        conf := orm.MemoryConf{
            DatabaseName:     "app_db",
            PersistenceMode:  orm.MemoryWithBackup,
            FilePath:         "./data/app_backup.db",
            AutoBackup:       true,
            BackupOnShutdown: true,
        }
        memDB, err = orm.InitMemoryDB(conf)
        if err == nil {
            db = memDB.GetDB()
        }
    default:
        return nil, fmt.Errorf("unsupported database type: %s", dbType)
    }
    
    if err != nil {
        return nil, err
    }
    
    return &DatabaseFactory{
        dbType: dbType,
        db:     db,
        memDB:  memDB,
    }, nil
}

func (f *DatabaseFactory) GetDB() *gorm.DB {
    return f.db
}

func (f *DatabaseFactory) GetMemoryDB() *orm.MemoryDB {
    return f.memDB
}

func (f *DatabaseFactory) Close() error {
    if f.memDB != nil {
        return f.memDB.Close()
    }
    return nil
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

## 持久化最佳实践

### 1. 开发环境
```go
// 使用纯内存模式，快速重启，不保留数据
conf := orm.MemoryConf{
    DatabaseName:    "dev_db",
    PersistenceMode: orm.PureMemo,
}
```

### 2. 测试环境
```go
// 使用内存+备份模式，测试数据持久化功能
conf := orm.MemoryConf{
    DatabaseName:     "test_db",
    PersistenceMode:  orm.MemoryWithBackup,
    FilePath:         "./test_data/backup.db",
    AutoBackup:       false, // 手动控制备份时机
    BackupOnShutdown: true,
}
```

### 3. 生产环境
```go
// 根据需求选择文件模式或MySQL
// 对于小规模应用，可以使用文件模式
conf := orm.MemoryConf{
    DatabaseName:    "prod_db",
    PersistenceMode: orm.FileMode,
    FilePath:        "./data/production.db",
}

// 对于大规模应用，使用MySQL
mysqlConf := orm.MysqlConf{
    DataBase: "production_db",
    Addr:     "mysql-server:3306",
    User:     "app_user",
    Password: os.Getenv("DB_PASSWORD"),
    Charset:  "utf8mb4",
}
```

## 注意事项

1. **内存数据库特点**：
   - **PureMemo**: 最快的性能，数据不持久化
   - **FileMode**: 数据持久化，性能略低于纯内存
   - **MemoryWithBackup**: 内存性能 + 数据安全，推荐用于中小型应用

2. **备份策略**：
   - 自动备份适合数据变化频繁的场景
   - 手动备份适合在关键操作后执行
   - 建议同时启用`BackupOnShutdown`确保数据安全

3. **数据库切换**：
   - 所有模式都使用相同的GORM接口
   - 事务管理器接口略有不同，建议使用工厂模式统一管理
   - 配置参数不同，需要根据模式设置相应配置

4. **监控和日志**：
   - 都支持Prometheus监控
   - 使用统一的zlog日志格式
   - 支持请求ID追踪
   - 自动备份过程会记录详细日志

5. **性能考虑**：
   - 内存模式读写性能最高
   - 文件模式适合数据量不大的持久化需求
   - 备份操作会暂时影响性能，建议在低峰期执行

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