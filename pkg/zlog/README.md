# 结构化日志

基于 Zap 封装的高性能结构化日志库，支持日志轮转、缓冲写入和多种配置选项。

## 功能特性

- ✅ **高性能**: 基于 uber-go/zap 高性能日志库
- ✅ **日志轮转**: 支持按日期自动轮转，自动清理过期日志
- ✅ **分级输出**: 普通日志和错误日志分文件存储
- ✅ **缓冲写入**: 可选的缓冲区提升写入性能
- ✅ **多种格式**: 支持 JSON 和 Console 两种输出格式
- ✅ **请求追踪**: 集成请求ID，便于链路追踪

## 快速开始

### 1. 基本使用

```go
package main

import (
    "github.com/xiangtao94/golib/pkg/zlog"
)

func main() {
    // 基本日志记录
    zlog.Info("应用启动成功")
    zlog.Error("发生错误", zlog.String("error", "database connection failed"))
    
    // 带字段的日志
    zlog.Info("用户登录", 
        zlog.String("username", "admin"),
        zlog.Int("user_id", 12345),
        zlog.Duration("login_time", time.Since(start)),
    )
}
```

### 2. 在 Gin 中使用

```go
func userHandler(c *gin.Context) {
    userID := c.Param("id")
    
    // 使用上下文日志，自动包含请求ID
    zlog.InfoLogger(c, "查询用户信息", 
        zlog.String("user_id", userID),
    )
    
    if err := getUserFromDB(userID); err != nil {
        zlog.ErrorLogger(c, "查询用户失败", 
            zlog.String("user_id", userID),
            zlog.String("error", err.Error()),
        )
        return
    }
}
```

## 日志字段类型

提供丰富的字段类型支持：

```go
// 字符串和字节
zlog.String("name", "张三")
zlog.ByteString("data", []byte("hello"))
zlog.Strings("tags", []string{"go", "web"})

// 数值类型
zlog.Int("count", 100)
zlog.Int64("timestamp", time.Now().Unix())
zlog.Float64("price", 99.99)
zlog.Uint("port", 8080)

// 布尔和时间
zlog.Bool("active", true)
zlog.Duration("cost", 100*time.Millisecond)

// 复杂对象
zlog.Any("user", userObject)
zlog.Reflect("config", appConfig)
```

## 日志配置

### 快速开始

```go
package main

import (
    "github.com/xiangtao94/golib/pkg/zlog"
)

func main() {
    // 方式1: 使用默认配置（推荐）
    logger := zlog.InitLog()
    
    // 方式2: 使用空配置（等同于默认配置）
    logger = zlog.InitLog(zlog.LogConfig{})
    
    // 方式3: 部分自定义配置
    logger = zlog.InitLog(zlog.LogConfig{
        Level: "debug",  // 只设置级别，其他使用默认值
    })
    
    // 使用日志
    logger.Info("应用启动成功")
}
```

### 默认配置

当不传入任何配置或传入空配置时，系统将使用以下默认配置：

```go
func DefaultLogConfig() LogConfig {
    return LogConfig{
        Level:     "info",                           // 日志级别
        Stdout:    true,                             // 控制台输出
        LogToFile: !env.IsDockerPlatform(),          // 容器环境默认不输出文件
        Format:    "json",                           // JSON格式
        LogDir:    "./log",                          // 日志目录
        Buffer: Buffer{
            Size:          256 * 1024,               // 256KB缓冲区
            FlushInterval: 5 * time.Second,          // 5秒刷新
        },
    }
}
```

### 完整配置示例

```go
logger := zlog.InitLog(zlog.LogConfig{
    Level:     "debug",            // 日志级别: debug, info, warn, error, fatal
    Stdout:    true,               // 是否输出到控制台
    LogToFile: true,               // 是否输出到文件
    Format:    "console",          // 输出格式: json, console
    LogDir:    "/var/log/myapp",   // 日志文件目录
    Buffer: zlog.Buffer{
        Switch:        "true",             // 缓冲区开关: true, false, 或空(自动判断)
        Size:          512 * 1024,         // 缓冲区大小(字节)
        FlushInterval: 10 * time.Second,   // 刷新间隔
    },
})
```

### 与Bootstrap集成

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib"
    "github.com/xiangtao94/golib/pkg/zlog"
)

func main() {
    engine := gin.New()
    
    // 使用默认日志配置
    golib.Bootstraps(engine,
        golib.WithAppName("my-app"),
        golib.WithZlog(),  // 不传参数，使用默认配置
    )
    
    // 或者使用自定义配置
    golib.Bootstraps(engine,
        golib.WithAppName("my-app"),
        golib.WithZlog(zlog.LogConfig{
            Level: "debug",
            Format: "console",
        }),
    )
    
    golib.StartHttpServer(engine, 8080)
}
```

### 配置说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| Level | string | "info" | 日志级别，支持: debug, info, warn, error, fatal |
| Stdout | bool | true | 是否输出到控制台 |
| LogToFile | bool | 环境判断 | 是否输出到文件，容器环境默认false，其他环境默认true |
| Format | string | "json" | 输出格式，支持: json, console |
| LogDir | string | "./log" | 日志文件目录 |
| Buffer.Switch | string | 环境判断 | 缓冲区开关，容器环境默认开启，其他环境默认关闭 |
| Buffer.Size | int | 262144 | 缓冲区大小(256KB) |
| Buffer.FlushInterval | time.Duration | 5s | 缓冲区刷新间隔 |

### 文件结构

```
logs/
├── app.log              # 当前日志软链接
├── app.log.wf           # 当前错误日志软链接  
├── app.log.access       # 当前访问日志软链接
├── app-2024-01-01.log   # 按日期轮转的日志
├── app-2024-01-01.log.wf
└── app-2024-01-01.log.access
```

## 性能优化

### 缓冲写入

启用缓冲区可以显著提升高并发场景下的日志写入性能：

```go
// 通过配置启用缓冲区
// BufferSwitch: true
// BufferSize: 256 * 1024        // 256KB 缓冲区
// BufferFlushInterval: 5s       // 5秒强制刷新
```

### 成本统计

提供便捷的执行时间统计：

```go
start := time.Now()
// ... 执行业务逻辑
zlog.Info("操作完成", zlog.AppendCostTime(start, time.Now())...)
```

## 请求追踪

### 自动请求ID

```go
// 中间件会自动生成和设置请求ID
func RequestIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        requestID := uuid.New().String()
        c.Set(zlog.ContextKeyRequestID, requestID)
        c.Next()
    }
}

// 在处理器中使用
func handler(c *gin.Context) {
    // 所有日志会自动包含请求ID
    zlog.InfoLogger(c, "处理请求")
}
```

### 获取请求ID

```go
requestID := zlog.GetRequestID(c)
```

## 完整示例

```go
package main

import (
    "time"
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/zlog"
)

func main() {
    r := gin.Default()
    
    // 请求ID中间件
    r.Use(func(c *gin.Context) {
        c.Set(zlog.ContextKeyRequestID, fmt.Sprintf("req-%d", time.Now().UnixNano()))
        c.Next()
    })
    
    r.GET("/api/users/:id", func(c *gin.Context) {
        start := time.Now()
        userID := c.Param("id")
        
        zlog.InfoLogger(c, "开始处理用户查询请求", 
            zlog.String("user_id", userID),
        )
        
        // 模拟业务处理
        time.Sleep(50 * time.Millisecond)
        
        if userID == "error" {
            zlog.ErrorLogger(c, "用户查询失败", 
                zlog.String("user_id", userID),
                zlog.String("reason", "用户不存在"),
                zlog.AppendCostTime(start, time.Now())...,
            )
            c.JSON(404, gin.H{"error": "用户不存在"})
            return
        }
        
        zlog.InfoLogger(c, "用户查询成功", 
            zlog.String("user_id", userID),
            zlog.AppendCostTime(start, time.Now())...,
        )
        
        c.JSON(200, gin.H{
            "id":   userID,
            "name": "用户" + userID,
        })
    })
    
    r.Run(":8080")
}
```

## 注意事项

- 日志文件按日轮转，自动保留14天
- 错误级别日志会同时输出到 `.log.wf` 文件
- 访问日志需要使用专门的 Access 日志记录器
- 缓冲写入可以提升性能，但可能在异常退出时丢失部分日志
- 请求ID会自动传递，便于分布式系统的链路追踪 