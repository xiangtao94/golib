# Gin 中间件集合

提供常用的 Gin 中间件，包括访问日志、CORS、压缩、限流、监控等功能。

## 功能特性

- ✅ **访问日志**: 详细的HTTP请求日志记录
- ✅ **CORS支持**: 跨域资源共享配置
- ✅ **Gzip压缩**: HTTP响应压缩中间件
- ✅ **限流控制**: 基于令牌桶的限流策略
- ✅ **监控指标**: Prometheus指标收集
- ✅ **异常恢复**: Panic恢复和错误处理
- ✅ **SSE支持**: 服务端推送事件配置
- ✅ **超时控制**: 请求超时处理
- ✅ **参数验证**: 请求参数验证中间件

## 中间件列表

| 中间件 | 文件 | 功能描述 |
|--------|------|----------|
| AccessLog | accesslog.go | HTTP访问日志记录 |
| CORS | cors.go | 跨域资源共享支持 |
| Gzip | gzip.go | HTTP响应压缩 |
| RateLimit | rate_limit.go | 请求频率限制 |
| Prometheus | prometheus.go | 指标监控收集 |
| Recover | recover.go | Panic异常恢复 |
| SSE | sse.go | 服务端推送事件 |
| Timeout | timeout.go | 请求超时控制 |
| Validator | validator.go | 参数验证 |

## 快速开始

### 基础中间件组合

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/middleware"
)

func main() {
    r := gin.Default()
    
    // 基础中间件
    r.Use(middleware.CORS())              // CORS支持
    r.Use(middleware.Recover())           // 异常恢复
    r.Use(middleware.AccessLog())         // 访问日志
    r.Use(middleware.Gzip())              // 响应压缩
    
    // 业务路由
    r.GET("/api/users", getUsersHandler)
    
    r.Run(":8080")
}
```

### 生产环境配置

```go
func setupMiddlewares(r *gin.Engine) {
    // 异常恢复（必须在最前面）
    r.Use(middleware.Recover())
    
    // CORS配置
    r.Use(middleware.CORS())
    
    // 访问日志
    r.Use(middleware.AccessLog())
    
    // 性能监控
    r.Use(middleware.Prometheus())
    
    // 请求压缩
    r.Use(middleware.Gzip())
    
    // 全局超时控制
    r.Use(middleware.Timeout(30 * time.Second))
    
    // API限流
    api := r.Group("/api")
    api.Use(middleware.RateLimit(100, time.Minute)) // 每分钟100次
    {
        api.GET("/users", getUsersHandler)
        api.POST("/users", createUserHandler)
    }
}
```

## 中间件详解

### AccessLog - 访问日志

记录详细的HTTP请求信息：

```go
r.Use(middleware.AccessLog())

// 日志包含：
// - 请求方法、路径、查询参数
// - 响应状态码、响应大小
// - 请求耗时、用户代理
// - 客户端IP、请求ID
```

### CORS - 跨域支持

```go
r.Use(middleware.CORS())

// 支持的响应头：
// Access-Control-Allow-Origin: *
// Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
// Access-Control-Allow-Headers: Origin, Content-Type, Authorization
```

### Gzip - 响应压缩

```go
r.Use(middleware.Gzip())

// 自动压缩响应内容，减少传输大小
// 支持的内容类型：text/*, application/json, application/xml等
```

### RateLimit - 请求限流

```go
// 每分钟最多100次请求
r.Use(middleware.RateLimit(100, time.Minute))

// 每秒最多10次请求
r.Use(middleware.RateLimit(10, time.Second))
```

### Prometheus - 指标监控

```go
r.Use(middleware.Prometheus())

// 收集的指标：
// - HTTP请求总数
// - 请求耗时分布
// - 响应状态码分布
// - 并发请求数
```

### Recover - 异常恢复

```go
r.Use(middleware.Recover())

// 功能：
// - 捕获panic异常
// - 记录错误堆栈
// - 返回500错误响应
// - 防止服务崩溃
```

### SSE - 服务端推送

```go
r.Use(middleware.SSE())

// 设置SSE相关响应头：
// Content-Type: text/event-stream
// Cache-Control: no-cache
// Connection: keep-alive
```

### Timeout - 超时控制

```go
// 30秒超时
r.Use(middleware.Timeout(30 * time.Second))

// 超时后自动取消请求并返回408状态码
```

### Validator - 参数验证

```go
r.Use(middleware.Validator())

// 在处理器中使用
func createUserHandler(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        // 验证错误会被中间件自动处理
        return
    }
    // 处理业务逻辑...
}
```

## 完整示例

```go
package main

import (
    "time"
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/middleware"
    "github.com/xiangtao94/golib/pkg/render"
)

func main() {
    r := gin.New() // 使用gin.New()避免默认中间件
    
    // 核心中间件（顺序很重要）
    r.Use(middleware.Recover())    // 1. 异常恢复
    r.Use(middleware.CORS())       // 2. CORS支持
    r.Use(middleware.AccessLog())  // 3. 访问日志
    r.Use(middleware.Prometheus()) // 4. 监控指标
    
    // 性能优化中间件
    r.Use(middleware.Gzip())                        // 响应压缩
    r.Use(middleware.Timeout(30 * time.Second))     // 全局超时
    
    // 健康检查（不需要限流）
    r.GET("/health", func(c *gin.Context) {
        render.RenderJsonSucc(c, gin.H{"status": "ok"})
    })
    
    // API路由组（有限流）
    api := r.Group("/api/v1")
    api.Use(middleware.RateLimit(1000, time.Minute)) // 每分钟1000次
    api.Use(middleware.Validator())                  // 参数验证
    {
        api.GET("/users", func(c *gin.Context) {
            // 模拟用户列表
            users := []map[string]interface{}{
                {"id": 1, "name": "张三"},
                {"id": 2, "name": "李四"},
            }
            render.RenderJsonSucc(c, users)
        })
        
        api.POST("/users", func(c *gin.Context) {
            var user map[string]interface{}
            if err := c.ShouldBindJSON(&user); err != nil {
                return // 验证中间件会处理错误
            }
            
            // 模拟创建用户
            user["id"] = 3
            render.RenderJsonSucc(c, user)
        })
    }
    
    // SSE路由（需要特殊处理）
    sse := r.Group("/sse")
    sse.Use(middleware.SSE())
    {
        sse.GET("/events", func(c *gin.Context) {
            // SSE事件推送
            for i := 0; i < 10; i++ {
                render.RenderStream(c, fmt.Sprintf("msg-%d", i), "message", map[string]interface{}{
                    "content": fmt.Sprintf("消息 %d", i+1),
                    "time":    time.Now().Format("15:04:05"),
                })
                time.Sleep(time.Second)
            }
        })
    }
    
    // 管理后台（更严格的限流）
    admin := r.Group("/admin")
    admin.Use(middleware.RateLimit(100, time.Minute)) // 每分钟100次
    {
        admin.GET("/stats", func(c *gin.Context) {
            render.RenderJsonSucc(c, gin.H{
                "requests": 12345,
                "users":    678,
            })
        })
    }
    
    r.Run(":8080")
}
```

## 中间件顺序

推荐的中间件使用顺序：

1. **Recover** - 必须在最前面，捕获所有panic
2. **CORS** - 处理跨域请求
3. **AccessLog** - 记录访问日志
4. **Prometheus** - 收集监控指标
5. **Gzip** - 响应压缩
6. **Timeout** - 超时控制
7. **RateLimit** - 请求限流
8. **Validator** - 参数验证
9. **SSE** - 特定路由使用

## 注意事项

- 中间件的执行顺序很重要，Recover必须在最前面
- 限流中间件会消耗内存，需要根据实际情况调整参数
- 监控中间件会收集详细指标，在高并发环境下注意性能影响
- SSE中间件只应用于需要服务端推送的路由
- 超时中间件会影响长连接和文件上传，需要合理设置时间 