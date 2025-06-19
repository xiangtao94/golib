# GoLib - Go语言企业级开发工具库

🚀 一个功能完整、开箱即用的 Go 语言企业级开发工具库，提供分层架构框架、数据库ORM、缓存、HTTP客户端、日志系统等完整的微服务开发解决方案。

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/xiangtao94/golib)](https://github.com/xiangtao94/golib/releases)

## ✨ 功能特性

- 🏗️ **分层架构框架** - 基于泛型的完整分层架构设计（Controller、Service、Dao、Api）
- 🗄️ **数据库支持** - 集成 GORM，支持 MySQL/PostgreSQL，提供连接池、事务、分表等高级功能
- 🔥 **缓存系统** - Redis 客户端封装，支持集群、哨兵模式，内置过期时间常量
- 🌐 **HTTP 客户端** - 高性能 HTTP 客户端，支持重试、负载均衡、流式处理
- 📝 **结构化日志** - 基于 Zap 的高性能日志系统，支持日志轮转、链路追踪
- 🛡️ **中间件集合** - 丰富的 Gin 中间件，包括 CORS、限流、监控、压缩等
- 🔧 **配置管理** - 基于 Viper 的多源配置读取，支持环境变量、配置文件
- 🌍 **多语言错误** - 统一的多语言错误处理机制
- 📊 **监控集成** - Prometheus 指标收集，Elasticsearch 日志存储
- 🎨 **响应渲染** - 统一的 HTTP 响应格式，支持 SSE 流式推送

## 📦 模块结构

```
golib/
├── flow/                    # 分层架构框架
│   ├── controller.go        # 控制器层
│   ├── service.go          # 服务层
│   ├── dao.go              # 数据访问层
│   ├── api.go              # 外部API层
│   └── layer.go            # 基础层接口
├── pkg/                    # 核心工具包
│   ├── elasticsearch/      # Elasticsearch客户端
│   ├── env/                # 配置管理
│   ├── errors/             # 多语言错误处理
│   ├── gcache/             # 内存缓存
│   ├── gimg/               # 图片服务
│   ├── http/               # HTTP客户端
│   ├── job/                # 定时任务
│   ├── mcp/                # MCP协议支持
│   ├── middleware/         # Gin中间件
│   ├── milvus/             # 向量数据库
│   ├── orm/                # MySQL ORM
│   ├── oss/                # 对象存储
│   ├── redis/              # Redis客户端
│   ├── render/             # 响应渲染
│   ├── utils/              # 工具函数
│   └── zlog/               # 结构化日志
└── bootstrap.go            # 应用启动器
```

## 🚀 快速开始

### 安装

```bash
go get github.com/xiangtao94/golib
```

### 最小化示例

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/flow"
    "github.com/xiangtao94/golib/pkg/middleware"
    "github.com/xiangtao94/golib/pkg/render"
)

// 请求结构体
type CreateUserRequest struct {
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
}

// 用户控制器
type UserController struct {
    flow.Controller
}

func (c *UserController) Action(req *CreateUserRequest) (any, error) {
    return map[string]interface{}{
        "id":    12345,
        "name":  req.Name, 
        "email": req.Email,
    }, nil
}

func main() {
    r := gin.Default()
    
    // 添加中间件
    r.Use(middleware.Recover())
    r.Use(middleware.AccessLog())
    r.Use(middleware.CORS())
    
    // 注册路由
    r.POST("/users", flow.Use(&UserController{}))
    
    r.Run(":8080")
}
```

### 完整企业级应用示例

```go
package main

import (
    "time"
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/flow"
    "github.com/xiangtao94/golib/pkg/env"
    "github.com/xiangtao94/golib/pkg/orm"
    "github.com/xiangtao94/golib/pkg/redis"
    "github.com/xiangtao94/golib/pkg/middleware"
    "github.com/xiangtao94/golib/pkg/zlog"
)

// 配置结构体
type Config struct {
    Server struct {
        Port int `yaml:"port"`
    } `yaml:"server"`
    Database orm.MysqlConf `yaml:"database"`
    Redis    redis.RedisConf `yaml:"redis"`
}

// 用户实体
type User struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Name      string    `gorm:"size:100;not null" json:"name"`
    Email     string    `gorm:"size:100;not null;unique" json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

func (User) TableName() string { return "users" }

// Dao层
type UserDao struct {
    flow.CommonDao[User]
}

func (d *UserDao) OnCreate() {
    d.SetTable("users")
}

// Service层
type UserService struct {
    flow.Service
}

func (s *UserService) CreateUser(req *CreateUserRequest) (*User, error) {
    userDao := flow.Create(s.GetCtx(), &UserDao{})
    
    user := &User{
        Name:  req.Name,
        Email: req.Email,
    }
    
    if err := userDao.Insert(user); err != nil {
        zlog.ErrorLogger(s.GetCtx(), "创建用户失败", 
            zlog.String("error", err.Error()))
        return nil, err
    }
    
    zlog.InfoLogger(s.GetCtx(), "用户创建成功", 
        zlog.Uint("user_id", user.ID))
    
    return user, nil
}

// Controller层
type UserController struct {
    flow.Controller
}

func (c *UserController) Action(req *CreateUserRequest) (any, error) {
    userService := flow.Create(c.GetCtx(), &UserService{})
    return userService.CreateUser(req)
}

func main() {
    // 加载配置
    var config Config
    if err := env.LoadConf("app", "production", &config); err != nil {
        panic("配置加载失败: " + err.Error())
    }
    
    // 初始化数据库
    db, err := orm.InitMysqlClient(config.Database)
    if err != nil {
        panic("数据库初始化失败: " + err.Error())
    }
    flow.SetDefaultDBClient(db)
    
    // 自动迁移
    db.AutoMigrate(&User{})
    
    // 初始化Redis
    redisClient, err := redis.InitRedisClient(config.Redis)
    if err != nil {
        panic("Redis初始化失败: " + err.Error())
    }
    
    // 初始化Gin
    r := gin.New()
    
    // 核心中间件
    r.Use(middleware.Recover())
    r.Use(middleware.CORS())
    r.Use(middleware.AccessLog())
    r.Use(middleware.Prometheus())
    r.Use(middleware.Gzip())
    
    // API路由
    api := r.Group("/api/v1")
    api.Use(middleware.RateLimit(1000, time.Minute))
    {
        api.POST("/users", flow.Use(&UserController{}))
    }
    
    zlog.Info("服务启动成功", zlog.Int("port", config.Server.Port))
    r.Run(fmt.Sprintf(":%d", config.Server.Port))
}
```

## 📚 核心模块说明

### 🏗️ Flow 分层架构

基于泛型的完整分层架构框架，提供清晰的职责分离：

```go
// Controller - HTTP请求处理
type UserController struct {
    flow.Controller
}

// Service - 业务逻辑
type UserService struct {
    flow.Service  
}

// Dao - 数据访问
type UserDao struct {
    flow.CommonDao[User]
}

// Api - 外部服务调用
type ThirdPartyApi struct {
    flow.Api
}
```

**特性：**
- 🔄 自动上下文传递
- 🎯 泛型类型安全
- 🔗 链式调用支持
- 📝 自动参数绑定
- 🗄️ 数据库事务支持

### 🗄️ 数据库 ORM

基于 GORM 的增强型 MySQL 客户端：

```go
// 基础配置
db, err := orm.InitMysqlClient(orm.MysqlConf{
    Addr:     "localhost:3306",
    User:     "root",
    Password: "password",
    DataBase: "myapp",
})

// 使用CommonDao获得完整CRUD功能
type UserDao struct {
    flow.CommonDao[User]
}

// 支持分表
dao.SetPartitionNum(10)
table := dao.GetPartitionTable(userID)
```

**特性：**
- ✅ 连接池管理
- ✅ 事务支持
- ✅ 读写分离  
- ✅ 分表分库
- ✅ Prometheus监控
- ✅ 自动日志记录

### 🔥 Redis 缓存

高性能 Redis 客户端，支持集群和哨兵模式：

```go
// 初始化客户端
client, err := redis.InitRedisClient(redis.RedisConf{
    Addr:     "localhost:6379",
    Password: "",
    MaxActive: 100,
})

// 使用预定义过期时间
client.Set(ctx, "user:123", userData, 
    time.Duration(redis.EXPIRE_TIME_1_HOUR)*time.Second)

// 键名自动前缀管理
prefix := redis.GetKeyPrefix() // "myapp:"
```

**特性：**
- 🔄 集群/哨兵支持
- ⏰ 预定义过期时间常量
- 🏷️ 自动键名前缀
- 🔗 连接池优化
- 📊 完整日志记录

### 🌐 HTTP 客户端

功能完整的 HTTP 客户端，支持多种编码和负载均衡：

```go
client := http.ClientConf{
    Domain:    "https://api.example.com",
    Timeout:   30 * time.Second,
    RetryTimes: 3,
}

// JSON请求
result, err := client.Post(ctx, http.RequestOptions{
    Path:        "/users",
    Encode:      http.EncodeJson,
    RequestBody: userData,
})

// 流式响应
client.PostStream(ctx, opts, func(data []byte) error {
    fmt.Printf("接收到数据: %s\n", string(data))
    return nil
})
```

**特性：**
- 🔄 自动重试机制
- ⚖️ 负载均衡支持
- 🌊 流式响应处理
- 📝 详细日志记录
- 🔧 灵活配置选项

### 📝 结构化日志

基于 Zap 的高性能日志系统：

```go
// 基础日志
zlog.Info("用户创建成功", 
    zlog.String("username", "admin"),
    zlog.Int("user_id", 12345))

// 上下文日志（自动包含请求ID）
zlog.InfoLogger(ctx, "处理用户请求",
    zlog.String("action", "create_user"))

// 错误日志
zlog.ErrorLogger(ctx, "数据库连接失败",
    zlog.String("error", err.Error()))
```

**特性：**
- 🚀 高性能写入
- 📅 自动日志轮转  
- 🔍 链路追踪支持
- 📊 结构化输出
- 💾 缓冲区优化

### 🛡️ 中间件集合

丰富的 Gin 中间件生态：

```go
r.Use(middleware.Recover())      // Panic恢复
r.Use(middleware.CORS())         // 跨域支持
r.Use(middleware.AccessLog())    // 访问日志
r.Use(middleware.Prometheus())   // 监控指标
r.Use(middleware.Gzip())         // 响应压缩
r.Use(middleware.RateLimit(100, time.Minute)) // 限流
r.Use(middleware.Timeout(30 * time.Second))   // 超时
```

## 🎨 配置示例

### 应用配置文件 `conf/production/app.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  addr: "localhost:3306"
  user: "root"
  password: "password"
  database: "myapp"
  maxIdleConns: 50
  maxOpenConns: 100
  connMaxLifeTime: 10m

redis:
  addr: "localhost:6379"
  db: 0
  maxActive: 100
  maxIdle: 20
  connTimeOut: 5s

elasticsearch:
  addr: "https://localhost:9200"
  username: "elastic"
  password: "password"

log:
  level: "info"
  path: "./logs"
  format: "json"
```

### 环境变量支持

```bash
# 应用配置
export MYAPP_SERVER_PORT=9000
export MYAPP_DATABASE_PASSWORD="secret123"
export MYAPP_REDIS_ADDR="redis-cluster:6379"

# 日志配置  
export MYAPP_LOG_LEVEL="debug"
export MYAPP_LOG_PATH="/var/logs/myapp"
```

## 🔧 高级功能

### 多数据库支持

```go
// 配置多个数据库实例
flow.SetNamedDBClient(map[string]*gorm.DB{
    "main_db":   mainDB,
    "log_db":    logDB,
    "cache_db":  cacheDB,
})

// 在Dao中使用指定数据库
func (d *LogDao) SaveLog(log *AccessLog) error {
    return d.GetDBByName("log_db").Create(log).Error
}
```

### 事务处理

```go
func (s *UserService) CreateUserWithProfile(userReq, profileReq) error {
    return s.GetDB().Transaction(func(tx *gorm.DB) error {
        userDao := flow.Create(s.GetCtx(), &UserDao{})
        userDao.SetDB(tx)
        
        profileDao := flow.Create(s.GetCtx(), &ProfileDao{})  
        profileDao.SetDB(tx)
        
        // 在事务中执行操作
        if err := userDao.Insert(user); err != nil {
            return err
        }
        return profileDao.Insert(profile)
    })
}
```

### 微服务通信

```go
// API层调用外部服务
type UserServiceApi struct {
    flow.Api
}

func (a *UserServiceApi) OnCreate() {
    a.Client = &http.ClientConf{
        Domain:  "http://user-service:8080",
        Timeout: 10 * time.Second,
    }
}

func (a *UserServiceApi) GetUserProfile(userID int) (*UserProfile, error) {
    res, err := a.ApiGet(fmt.Sprintf("/users/%d/profile", userID), nil)
    if err != nil {
        return nil, err
    }
    
    var profile UserProfile
    return &profile, a.DecodeApiResponse(&profile, res, err)
}
```

### 监控和observability

```go
// Prometheus指标自动收集
r.Use(middleware.Prometheus())

// 自定义指标
prometheus.MustRegister(orm.MysqlPromCollector)

// Elasticsearch日志存储
esClient, _ := elasticsearch.InitESClient(esConf)
esClient.DocumentInsert(ctx, "app-logs", logDocs)

// 链路追踪
zlog.InfoLogger(ctx, "处理请求开始", zlog.String("trace_id", traceID))
```

## 📖 文档链接

- [Flow 分层架构](./flow/README.md) - 分层架构框架使用指南
- [配置管理](./pkg/env/README.md) - 基于Viper的配置管理
- [数据库ORM](./pkg/orm/README.md) - MySQL数据库访问
- [Redis缓存](./pkg/redis/README.md) - Redis客户端使用
- [HTTP客户端](./pkg/http/README.md) - HTTP客户端配置
- [结构化日志](./pkg/zlog/README.md) - 日志系统使用
- [中间件](./pkg/middleware/README.md) - Gin中间件集合
- [错误处理](./pkg/errors/README.md) - 多语言错误处理
- [响应渲染](./pkg/render/README.md) - HTTP响应格式化

## 🤝 贡献指南

我们欢迎所有形式的贡献！

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

### 开发环境

```bash
# 克隆仓库
git clone https://github.com/xiangtao94/golib.git
cd golib

# 安装依赖
go mod tidy

# 运行测试
go test ./...

# 代码格式化
go fmt ./...
```

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

感谢以下优秀的开源项目：

- [Gin](https://github.com/gin-gonic/gin) - HTTP web框架
- [GORM](https://github.com/go-gorm/gorm) - ORM库
- [Zap](https://github.com/uber-go/zap) - 结构化日志
- [Viper](https://github.com/spf13/viper) - 配置管理
- [Redis](https://github.com/redis/go-redis) - Redis客户端
- [Prometheus](https://github.com/prometheus/client_golang) - 监控指标

## 📞 联系方式

- 作者: xiangtao
- 邮箱: xiangtao1994@gmail.com
- GitHub: [@xiangtao94](https://github.com/xiangtao94)

---

⭐ 如果这个项目对你有帮助，请给个 Star！