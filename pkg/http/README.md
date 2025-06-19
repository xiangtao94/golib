# HTTP 客户端

基于 `resty` 封装的高性能 HTTP 客户端，提供完整的日志记录、重试机制、负载均衡等功能。

## 功能特性

- ✅ **多种编码支持**: JSON、Form、Raw、File 等编码方式
- ✅ **负载均衡**: 支持多域名轮询和自定义负载均衡策略
- ✅ **重试机制**: 可配置的重试次数和重试策略
- ✅ **流式处理**: 支持 GET/POST 流式响应处理
- ✅ **完整日志**: 集成 zlog 记录详细的请求/响应信息
- ✅ **连接池优化**: 优化的连接池配置和代理支持
- ✅ **超时控制**: 全局和单次请求的超时时间控制

## 快速开始

### 1. 配置客户端

```go
type ClientConf struct {
    Service          string                   `yaml:"service"`          // 服务名称
    Domain           string                   `yaml:"domain"`           // 单个域名
    Domains          []string                 `yaml:"domains"`          // 多个域名（负载均衡）
    Timeout          time.Duration            `yaml:"timeout"`          // 请求超时时间
    ConnectTimeout   time.Duration            `yaml:"connectTimeout"`   // 连接超时时间
    MaxReqBodyLen    int                      `yaml:"maxReqBodyLen"`    // 请求体最大展示长度
    MaxRespBodyLen   int                      `yaml:"maxRespBodyLen"`   // 响应体最大展示长度
    HttpStat         bool                     `yaml:"httpStat"`         // HTTP 分析开关
    RetryTimes       int                      `yaml:"retryTimes"`       // 最大重试次数
    RetryWaitTime    time.Duration            `yaml:"retryWaitTime"`    // 重试等待时间
    RetryMaxWaitTime time.Duration            `yaml:"retryMaxWaitTime"` // 最大重试等待时间
    Proxy            string                   `yaml:"proxy"`            // 代理地址
}
```

### 2. 初始化客户端

```go
package main

import (
    "time"
    "github.com/xiangtao94/golib/pkg/http"
)

func main() {
    conf := http.ClientConf{
        Service:        "user-service",
        Domain:         "https://api.example.com",
        Timeout:        30 * time.Second,
        RetryTimes:     3,
        RetryWaitTime:  500 * time.Millisecond,
        MaxReqBodyLen:  1024,
        MaxRespBodyLen: 10240,
    }
}
```

## 编码方式

| 编码类型 | 常量 | 说明 |
|----------|------|------|
| JSON | `EncodeJson` | JSON 格式编码 |
| Form | `EncodeForm` | 表单格式编码 |
| Raw | `EncodeRaw` | 原始字符串 |
| Raw Byte | `EncodeRawByte` | 原始字节数组 |
| File | `EncodeFile` | 文件上传 |

## 使用示例

### GET 请求

```go
// 简单 GET 请求
opts := http.RequestOptions{
    Path: "/users/123",
    QueryParams: map[string]string{
        "include": "profile",
        "format":  "json",
    },
    Headers: map[string]string{
        "Authorization": "Bearer token",
    },
}

result, err := conf.Get(ctx, opts)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("状态码: %d\n", result.HttpCode)
fmt.Printf("响应: %s\n", string(result.Response))
```

### POST 请求

```go
// JSON POST 请求
userData := map[string]interface{}{
    "name":  "张三",
    "email": "zhangsan@example.com",
    "age":   25,
}

opts := http.RequestOptions{
    Path:        "/users",
    Encode:      http.EncodeJson,
    RequestBody: userData,
    Headers: map[string]string{
        "Content-Type": "application/json",
    },
}

result, err := conf.Post(ctx, opts)
if err != nil {
    log.Fatal(err)
}
```

### Form 表单请求

```go
// 表单 POST 请求
formData := map[string]string{
    "username": "admin",
    "password": "secret",
}

opts := http.RequestOptions{
    Path:        "/auth/login",
    Encode:      http.EncodeForm,
    RequestBody: formData,
}

result, err := conf.Post(ctx, opts)
```

### 文件上传

```go
// 文件上传请求
opts := http.RequestOptions{
    Path:   "/upload",
    Encode: http.EncodeFile,
    RequestFiles: map[string][]string{
        "avatar": {"/path/to/avatar.jpg"},
        "docs":   {"/path/to/doc1.pdf", "/path/to/doc2.pdf"},
    },
    RequestBody: map[string]string{
        "description": "用户头像和文档",
    },
}

result, err := conf.Post(ctx, opts)
```

### 流式响应处理

```go
// GET 流式响应
opts := http.RequestOptions{
    Path: "/stream/data",
}

// 定义数据处理函数
dataHandler := func(data []byte) error {
    fmt.Printf("接收到数据: %s\n", string(data))
    return nil
}

result, err := conf.GetStream(ctx, opts, dataHandler)
if err != nil {
    log.Fatal(err)
}

// POST 流式响应
result, err = conf.PostStream(ctx, opts, dataHandler)
```

### 负载均衡配置

```go
// 多域名负载均衡
conf := http.ClientConf{
    Service: "api-service",
    Domains: []string{
        "https://api1.example.com",
        "https://api2.example.com", 
        "https://api3.example.com",
    },
    Timeout:    30 * time.Second,
    RetryTimes: 3,
}

// 客户端会自动在多个域名间轮询
result, err := conf.Get(ctx, http.RequestOptions{
    Path: "/health",
})
```

### 自定义重试策略

```go
// 自定义重试条件
conf.RetryPolicy = func(r *resty.Response, err error) bool {
    // 只有在 5xx 错误或网络错误时才重试
    return r.StatusCode() >= 500 || err != nil
}

result, err := conf.Get(ctx, opts)
```

## 日志记录

客户端会自动记录以下信息：

- 请求URL、方法、头部、参数
- 响应状态码、头部、内容
- 请求耗时
- 错误信息（如果有）
- 请求ID（集成 Gin 框架）

日志输出长度可以通过配置控制：

```go
conf := http.ClientConf{
    MaxReqBodyLen:  1024,  // 请求体最大展示1024字符
    MaxRespBodyLen: 10240, // 响应体最大展示10240字符
}

// 特殊值：
// -1: 不打印对应内容
//  0: 使用默认长度（10240）
```

## 完整示例

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/http"
)

func main() {
    // 配置HTTP客户端
    conf := http.ClientConf{
        Service:        "user-api",
        Domain:         "https://jsonplaceholder.typicode.com",
        Timeout:        30 * time.Second,
        RetryTimes:     3,
        RetryWaitTime:  500 * time.Millisecond,
        MaxReqBodyLen:  1024,
        MaxRespBodyLen: 10240,
    }
    
    r := gin.Default()
    
    r.GET("/users/:id", func(c *gin.Context) {
        userID := c.Param("id")
        
        // 调用外部API
        opts := http.RequestOptions{
            Path: "/users/" + userID,
            Headers: map[string]string{
                "Accept": "application/json",
            },
        }
        
        result, err := conf.Get(c, opts)
        if err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        
        c.Data(result.HttpCode, "application/json", result.Response)
    })
    
    r.Run(":8080")
}
```

## 注意事项

- 客户端使用连接池，无需频繁创建销毁
- 流式响应处理适用于大数据传输场景
- 重试机制默认针对网络错误，可自定义重试条件
- 日志记录会自动截断过长的内容以避免日志文件过大 