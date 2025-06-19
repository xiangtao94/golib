# Flow 分层架构框架

提供基于泛型的分层架构框架，支持 Controller、Service、Dao、Api、Data 等分层模式，集成 Gin 框架和 GORM。

## 功能特性

- ✅ **分层架构**: 清晰的分层设计模式（Controller、Service、Dao、Api、Data）
- ✅ **泛型支持**: 基于 Go 泛型提供类型安全的开发体验
- ✅ **上下文传递**: 自动传递 Gin 上下文到各个层级
- ✅ **数据库集成**: 深度集成 GORM，支持多数据库实例
- ✅ **HTTP 客户端**: 内置 HTTP 客户端支持外部 API 调用
- ✅ **自动绑定**: 自动参数绑定和错误处理
- ✅ **链式调用**: 支持层级间的流畅调用

## 架构层级

```
┌─────────────┐
│ Controller  │  HTTP 请求处理层
└─────────────┘
       │
┌─────────────┐
│   Service   │  业务逻辑层
└─────────────┘
       │
┌─────────────┐
│    Dao      │  数据访问层
└─────────────┘
       │
┌─────────────┐
│   数据库     │  MySQL/PostgreSQL等
└─────────────┘

┌─────────────┐
│     Api     │  外部API调用层
└─────────────┘
       │
┌─────────────┐
│   外部服务   │  第三方API服务
└─────────────┘

┌─────────────┐
│    Data     │  数据处理层
└─────────────┘
```

## 快速开始

### 1. Controller 层使用

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/flow"
)

// 定义请求结构体
type CreateUserRequest struct {
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
    Age   int    `json:"age" binding:"min=1,max=120"`
}

// 定义响应结构体
type CreateUserResponse struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 用户控制器
type UserController struct {
    flow.Controller
}

func (c *UserController) Action(req *CreateUserRequest) (any, error) {
    // 业务逻辑处理
    response := &CreateUserResponse{
        ID:    12345,
        Name:  req.Name,
        Email: req.Email,
    }
    return response, nil
}

func main() {
    r := gin.Default()
    
    // 注册路由
    r.POST("/users", flow.Use(&UserController{}))
    
    r.Run(":8080")
}
```

### 2. Service 层使用

```go
// 用户服务
type UserService struct {
    flow.Service
}

func (s *UserService) CreateUser(req *CreateUserRequest) (*CreateUserResponse, error) {
    // 业务逻辑处理
    // 可以调用其他服务、Dao层等
    
    return &CreateUserResponse{
        ID:    12345,
        Name:  req.Name,
        Email: req.Email,
    }, nil
}

// 在 Controller 中使用 Service
func (c *UserController) Action(req *CreateUserRequest) (any, error) {
    userService := flow.Create(c.GetCtx(), &UserService{})
    return userService.CreateUser(req)
}
```

### 3. Dao 层使用

```go
// 用户实体
type User struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string    `gorm:"size:100;not null"`
    Email     string    `gorm:"size:100;not null;unique"`
    Age       int       `gorm:"default:0"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
    UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (User) TableName() string {
    return "users"
}

// 用户Dao
type UserDao struct {
    flow.CommonDao[User]
}

func (d *UserDao) OnCreate() {
    d.SetTable("users")
}

func (d *UserDao) GetByEmail(email string) (*User, error) {
    var user User
    err := d.GetDB().Where("email = ?", email).First(&user).Error
    if err != nil {
        return nil, err
    }
    return &user, nil
}

// 在 Service 中使用 Dao
func (s *UserService) CreateUser(req *CreateUserRequest) (*CreateUserResponse, error) {
    userDao := flow.Create(s.GetCtx(), &UserDao{})
    
    // 检查邮箱是否已存在
    existing, _ := userDao.GetByEmail(req.Email)
    if existing != nil {
        return nil, errors.New("邮箱已存在")
    }
    
    // 创建新用户
    user := &User{
        Name:  req.Name,
        Email: req.Email,
        Age:   req.Age,
    }
    
    if err := userDao.Insert(user); err != nil {
        return nil, err
    }
    
    return &CreateUserResponse{
        ID:    int(user.ID),
        Name:  user.Name,
        Email: user.Email,
    }, nil
}
```

### 4. Api 层使用

```go
// 外部API服务
type ThirdPartyApi struct {
    flow.Api
}

func (a *ThirdPartyApi) OnCreate() {
    // 设置HTTP客户端配置
    a.Client = &http.ClientConf{
        Domain:  "https://api.thirdparty.com",
        Timeout: 30 * time.Second,
    }
    a.EncodeType = http.EncodeJson
}

func (a *ThirdPartyApi) SendWelcomeEmail(email, name string) error {
    reqBody := map[string]interface{}{
        "to":      email,
        "subject": "欢迎注册",
        "body":    fmt.Sprintf("欢迎 %s 注册我们的服务！", name),
    }
    
    res, err := a.ApiPost("/send-email", reqBody)
    if err != nil {
        return err
    }
    
    if res.Code != 200 {
        return errors.New(res.Message)
    }
    
    return nil
}

// 在 Service 中使用 Api
func (s *UserService) CreateUser(req *CreateUserRequest) (*CreateUserResponse, error) {
    // ... 创建用户逻辑
    
    // 发送欢迎邮件
    thirdPartyApi := flow.Create(s.GetCtx(), &ThirdPartyApi{})
    if err := thirdPartyApi.SendWelcomeEmail(req.Email, req.Name); err != nil {
        zlog.Errorf(s.GetCtx(), "发送欢迎邮件失败: %v", err)
        // 不影响用户创建，只记录日志
    }
    
    return response, nil
}
```

## 数据库配置

### 初始化数据库连接

```go
package main

import (
    "github.com/xiangtao94/golib/flow"
    "github.com/xiangtao94/golib/pkg/orm"
)

func initDatabase() {
    // 主数据库
    masterDB, err := orm.InitMysqlClient(orm.MysqlConf{
        Addr:     "localhost:3306",
        User:     "root",
        Password: "password",
        DataBase: "main_db",
    })
    if err != nil {
        log.Fatal("初始化主数据库失败:", err)
    }
    
    // 设置默认数据库
    flow.SetDefaultDBClient(masterDB)
    
    // 配置多个命名数据库
    flow.SetNamedDBClient(map[string]*gorm.DB{
        "log_db":   logDB,
        "cache_db": cacheDB,
    })
}
```

### 使用多数据库

```go
type LogDao struct {
    flow.Dao
}

func (d *LogDao) OnCreate() {
    d.SetTable("access_logs")
}

func (d *LogDao) SaveToLogDB(log *AccessLog) error {
    // 使用命名数据库
    return d.GetDBByName("log_db").Create(log).Error
}

func (d *LogDao) SaveToMainDB(user *User) error {
    // 使用默认数据库
    return d.GetDB().Create(user).Error
}
```

## 高级特性

### 分表支持

```go
type OrderDao struct {
    flow.CommonDao[Order]
}

func (d *OrderDao) OnCreate() {
    d.SetTable("orders_")
    d.SetPartitionNum(10) // 10个分表
}

func (d *OrderDao) GetOrderByUserId(userId int64) (*Order, error) {
    // 根据用户ID自动选择分表
    tableName := d.GetPartitionTable(userId)
    
    var order Order
    err := d.GetDB().Table(tableName).Where("user_id = ?", userId).First(&order).Error
    return &order, err
}
```

### 读写分离

```go
func (d *UserDao) GetUserProfile(userId int) (*User, error) {
    // 强制从主库读取
    d.SetReadDbMaster(true)
    defer d.SetReadDbMaster(false)
    
    var user User
    err := d.GetDB().Where("id = ?", userId).First(&user).Error
    return &user, err
}
```

### 事务支持

```go
func (s *UserService) CreateUserWithProfile(userReq *CreateUserRequest, profileReq *CreateProfileRequest) error {
    // 开启事务
    return s.GetDB().Transaction(func(tx *gorm.DB) error {
        userDao := flow.Create(s.GetCtx(), &UserDao{})
        userDao.SetDB(tx) // 使用事务连接
        
        // 创建用户
        user := &User{Name: userReq.Name, Email: userReq.Email}
        if err := userDao.Insert(user); err != nil {
            return err
        }
        
        profileDao := flow.Create(s.GetCtx(), &ProfileDao{})
        profileDao.SetDB(tx) // 使用相同事务连接
        
        // 创建用户档案
        profile := &Profile{UserID: user.ID, Bio: profileReq.Bio}
        if err := profileDao.Insert(profile); err != nil {
            return err
        }
        
        return nil
    })
}
```

### 自定义绑定器

```go
type FileUploadController struct {
    flow.Controller
}

// 使用表单绑定而不是JSON
func (c *FileUploadController) RequestBind() binding.Binding {
    return binding.FormMultipart
}

func (c *FileUploadController) Action(req *FileUploadRequest) (any, error) {
    // 处理文件上传
    file, err := c.GetCtx().FormFile("upload")
    if err != nil {
        return nil, err
    }
    
    // 保存文件逻辑...
    return gin.H{"message": "文件上传成功"}, nil
}
```

## 完整示例

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/flow"
    "github.com/xiangtao94/golib/pkg/orm"
    "github.com/xiangtao94/golib/pkg/middleware"
)

// 实体定义
type User struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Name      string    `gorm:"size:100;not null" json:"name"`
    Email     string    `gorm:"size:100;not null;unique" json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

func (User) TableName() string { return "users" }

// 请求/响应结构体
type CreateUserRequest struct {
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
}

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
        return nil, err
    }
    
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
    // 初始化数据库
    db, err := orm.InitMysqlClient(orm.MysqlConf{
        Addr:     "localhost:3306",
        User:     "root", 
        Password: "password",
        DataBase: "testdb",
    })
    if err != nil {
        log.Fatal(err)
    }
    flow.SetDefaultDBClient(db)
    
    // 自动迁移
    db.AutoMigrate(&User{})
    
    // 初始化Gin
    r := gin.Default()
    
    // 添加中间件
    r.Use(middleware.Recover())
    r.Use(middleware.AccessLog())
    
    // 注册路由
    api := r.Group("/api/v1")
    {
        api.POST("/users", flow.Use(&UserController{}))
    }
    
    r.Run(":8080")
}
```

## 最佳实践

### 1. 分层职责

- **Controller**: 只负责参数绑定和响应渲染，不包含业务逻辑
- **Service**: 承载核心业务逻辑，可以调用多个Dao或Api
- **Dao**: 只负责数据库操作，不包含业务逻辑
- **Api**: 只负责外部API调用，处理HTTP通信

### 2. 错误处理

```go
func (s *UserService) CreateUser(req *CreateUserRequest) (*User, error) {
    userDao := flow.Create(s.GetCtx(), &UserDao{})
    
    // 使用业务错误码
    if err := s.validateEmail(req.Email); err != nil {
        return nil, errors.ErrorParamInvalid
    }
    
    // 记录详细日志
    zlog.InfoLogger(s.GetCtx(), "开始创建用户", 
        zlog.String("email", req.Email))
    
    user := &User{Name: req.Name, Email: req.Email}
    if err := userDao.Insert(user); err != nil {
        zlog.ErrorLogger(s.GetCtx(), "创建用户失败", 
            zlog.String("email", req.Email),
            zlog.String("error", err.Error()))
        return nil, errors.ErrorSystemError
    }
    
    return user, nil
}
```

### 3. 上下文传递

```go
// 正确方式：使用 flow.Create 创建新实例
func (s *UserService) processUser() {
    userDao := flow.Create(s.GetCtx(), &UserDao{})
    // userDao 会自动获得相同的上下文
}

// 错误方式：直接实例化
func (s *UserService) processUserWrong() {
    userDao := &UserDao{} // 缺少上下文信息
}
```

## 注意事项

- 各层之间通过 `flow.Create` 创建实例，自动传递上下文
- Controller 的 Action 方法必须返回 `(any, error)`
- Dao 层建议继承 `CommonDao[T]` 获得基础CRUD功能
- 使用事务时需要通过 `SetDB(tx)` 传递事务连接
- Api 层需要在 `OnCreate` 中配置 HTTP 客户端
- 所有层都会自动记录请求ID，便于链路追踪 