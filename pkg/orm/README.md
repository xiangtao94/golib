# MySQL ORM

基于 GORM 封装的 MySQL 数据库访问层，提供完整的日志记录、连接池管理和 Prometheus 监控功能。

## 功能特性

- ✅ **GORM 集成**: 基于流行的 GORM ORM 框架
- ✅ **连接池管理**: 优化的数据库连接池配置
- ✅ **日志记录**: 集成 zlog 记录所有 SQL 操作
- ✅ **性能监控**: 内置 Prometheus 指标收集
- ✅ **标准模型**: 提供通用的 CRUD 模型结构
- ✅ **分页支持**: 内置分页查询功能
- ✅ **超时控制**: 可配置的连接、读写超时

## 快速开始

### 1. 配置结构体

```go
type MysqlConf struct {
    DataBase        string        `yaml:"database"`        // 数据库名
    Addr            string        `yaml:"addr"`            // 数据库地址
    User            string        `yaml:"user"`            // 用户名
    Password        string        `yaml:"password"`        // 密码
    Charset         string        `yaml:"charset"`         // 字符集
    MaxIdleConns    int           `yaml:"maxidleconns"`    // 最大空闲连接数
    MaxOpenConns    int           `yaml:"maxopenconns"`    // 最大打开连接数
    ConnMaxIdlTime  time.Duration `yaml:"maxIdleTime"`     // 连接最大空闲时间
    ConnMaxLifeTime time.Duration `yaml:"connMaxLifeTime"` // 连接最大生存时间
    ConnTimeOut     time.Duration `yaml:"connTimeOut"`     // 连接超时时间
    WriteTimeOut    time.Duration `yaml:"writeTimeOut"`    // 写超时时间
    ReadTimeOut     time.Duration `yaml:"readTimeOut"`     // 读超时时间
}
```

### 2. 初始化数据库连接

```go
package main

import (
    "time"
    "github.com/xiangtao94/golib/pkg/orm"
    "gorm.io/gorm"
)

func main() {
    conf := orm.MysqlConf{
        DataBase:        "myapp",
        Addr:            "localhost:3306",
        User:            "root",
        Password:        "password",
        Charset:         "utf8mb4",
        MaxIdleConns:    50,
        MaxOpenConns:    100,
        ConnMaxIdlTime:  5 * time.Minute,
        ConnMaxLifeTime: 10 * time.Minute,
        ConnTimeOut:     3 * time.Second,
        WriteTimeOut:    1200 * time.Millisecond,
        ReadTimeOut:     1200 * time.Millisecond,
    }
    
    db, err := orm.InitMysqlClient(conf)
    if err != nil {
        log.Fatal(err)
    }
    
    // 使用数据库连接
    // ...
}
```

## 标准模型

### CrudModel 基础模型

```go
type CrudModel struct {
    CreatedAt time.Time      `json:"createdAt" gorm:"comment:创建时间"`
    UpdatedAt time.Time      `json:"updatedAt" gorm:"comment:最后更新时间"`
    DeletedAt gorm.DeletedAt `json:"-" gorm:"index;comment:删除时间"`
}

// 使用示例
type User struct {
    ID       uint   `gorm:"primary_key"`
    Username string `gorm:"size:100;not null;unique"`
    Email    string `gorm:"size:100;not null"`
    orm.CrudModel // 嵌入标准字段
}
```

### 分页结构

```go
type NormalPage struct {
    No      int    // 当前第几页
    Size    int    // 每页大小
    OrderBy string `json:"orderBy"` // 排序规则
}

type Option struct {
    IsNeedCnt  bool `json:"isNeedCnt"`  // 是否需要总数
    IsNeedPage bool `json:"isNeedPage"` // 是否需要分页
}
```

## 使用示例

### 基本 CRUD 操作

```go
// 定义模型
type User struct {
    ID       uint   `gorm:"primary_key"`
    Username string `gorm:"size:100;not null;unique"`
    Email    string `gorm:"size:100;not null"`
    Age      int    `gorm:"default:0"`
    orm.CrudModel
}

// 创建表
err := db.AutoMigrate(&User{})
if err != nil {
    log.Fatal(err)
}

// 创建记录
user := User{
    Username: "zhangsan",
    Email:    "zhangsan@example.com",
    Age:      25,
}
result := db.Create(&user)
if result.Error != nil {
    log.Fatal(result.Error)
}

// 查询记录
var users []User
db.Where("age > ?", 18).Find(&users)

// 更新记录
db.Model(&user).Update("Age", 26)

// 删除记录（软删除）
db.Delete(&user)
```

### 分页查询

```go
// 使用内置分页函数
page := &orm.NormalPage{
    No:      1,    // 第1页
    Size:    10,   // 每页10条
    OrderBy: "created_at desc", // 按创建时间降序
}

var users []User
var total int64

// 获取总数
db.Model(&User{}).Count(&total)

// 分页查询
db.Scopes(orm.NormalPaginate(page)).Find(&users)

fmt.Printf("总数: %d, 当前页: %d, 每页: %d\n", total, page.No, page.Size)
```

### 高级查询示例

```go
// 条件查询
var users []User
db.Where("age BETWEEN ? AND ?", 18, 65).
   Where("email LIKE ?", "%@gmail.com").
   Order("created_at desc").
   Find(&users)

// 关联查询
type Profile struct {
    ID     uint   `gorm:"primary_key"`
    UserID uint   `gorm:"not null"`
    Bio    string `gorm:"type:text"`
    User   User   `gorm:"foreignKey:UserID"`
}

var profiles []Profile
db.Preload("User").Find(&profiles)

// 聚合查询
type AgeStats struct {
    AvgAge   float64
    MinAge   int
    MaxAge   int
    UserCount int64
}

var stats AgeStats
db.Model(&User{}).
   Select("AVG(age) as avg_age, MIN(age) as min_age, MAX(age) as max_age, COUNT(*) as user_count").
   Scan(&stats)
```

### 事务处理

```go
// 手动事务
tx := db.Begin()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
    }
}()

if err := tx.Error; err != nil {
    return err
}

// 在事务中执行操作
if err := tx.Create(&user1).Error; err != nil {
    tx.Rollback()
    return err
}

if err := tx.Create(&user2).Error; err != nil {
    tx.Rollback()
    return err
}

// 提交事务
return tx.Commit().Error

// 或者使用 GORM 的事务函数
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&user1).Error; err != nil {
        return err
    }
    if err := tx.Create(&user2).Error; err != nil {
        return err
    }
    return nil
})
```

## 性能监控

客户端自动集成 Prometheus 监控指标：

```go
// Prometheus 收集器会自动注册
// 可以通过以下方式访问：
collector := orm.MysqlPromCollector

// 在 Prometheus 中注册
prometheus.MustRegister(collector)
```

监控指标包括：
- 数据库连接数
- 空闲连接数
- 正在使用的连接数
- 等待连接数
- 连接持续时间等

## 日志记录

所有 SQL 操作都会自动记录日志，包括：

- SQL 语句
- 执行时间
- 影响行数
- 错误信息（如果有）
- 请求ID（集成 Gin 框架）

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