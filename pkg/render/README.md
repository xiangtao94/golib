# HTTP 响应渲染

提供统一的 HTTP 响应格式化和渲染功能，支持 JSON、SSE 流式响应和自定义渲染器。

## 功能特性

- ✅ **统一响应格式**: 标准化的 JSON 响应结构
- ✅ **多语言错误**: 集成多语言错误处理机制
- ✅ **SSE 流式响应**: 支持服务端推送事件
- ✅ **自定义渲染器**: 支持注册自定义响应格式
- ✅ **错误栈追踪**: 自动记录和格式化错误堆栈
- ✅ **请求ID追踪**: 自动添加请求ID到响应头

## 默认响应格式

```go
type DefaultRender struct {
    Code      int         `json:"code" example:"200"`
    Message   string      `json:"message" example:"Success"`
    RequestId string      `json:"request_id,omitempty"`
    Data      interface{} `json:"data"`
}
```

## 快速开始

### 1. 基本使用

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/render"
    "github.com/xiangtao94/golib/pkg/errors"
)

func main() {
    r := gin.Default()
    
    // 成功响应
    r.GET("/success", func(c *gin.Context) {
        data := map[string]interface{}{
            "name": "张三",
            "age":  25,
        }
        render.RenderJsonSucc(c, data)
    })
    
    // 错误响应
    r.GET("/error", func(c *gin.Context) {
        err := errors.ErrorParamInvalid
        render.RenderJsonFail(c, err)
    })
    
    r.Run(":8080")
}
```

### 2. 自定义渲染器

```go
// 定义自定义渲染器
type CustomRender struct {
    Status  string      `json:"status"`
    Result  interface{} `json:"result"`
    TraceId string      `json:"trace_id"`
}

func (r *CustomRender) SetReturnCode(code int) {
    if code == 200 {
        r.Status = "success"
    } else {
        r.Status = "error"
    }
}

func (r *CustomRender) SetReturnMsg(msg string) {
    // 可以根据需要处理消息
}

func (r *CustomRender) SetReturnData(data interface{}) {
    r.Result = data
}

func (r *CustomRender) SetReturnRequestId(requestId string) {
    r.TraceId = requestId
}

func (r *CustomRender) GetReturnCode() int {
    if r.Status == "success" {
        return 200
    }
    return 500
}

func (r *CustomRender) GetReturnMsg() string {
    return r.Status
}

// 注册自定义渲染器
func init() {
    render.RegisterRender(func() render.Render {
        return &CustomRender{}
    })
}
```

## API 参考

### 成功响应

```go
// 返回成功响应
func RenderJsonSucc(ctx *gin.Context, data interface{})
```

**示例:**
```go
r.GET("/users", func(c *gin.Context) {
    users := []User{{Name: "张三"}, {Name: "李四"}}
    render.RenderJsonSucc(c, users)
})

// 响应格式:
// {
//   "code": 200,
//   "message": "success", 
//   "request_id": "req-123",
//   "data": [{"name": "张三"}, {"name": "李四"}]
// }
```

### 错误响应

```go
// 返回错误响应
func RenderJsonFail(ctx *gin.Context, err error)
```

**示例:**
```go
r.GET("/validate", func(c *gin.Context) {
    if someCondition {
        render.RenderJsonFail(c, errors.ErrorParamInvalid)
        return
    }
    render.RenderJsonSucc(c, gin.H{"valid": true})
})

// 错误响应格式:
// {
//   "code": 2,
//   "message": "请求参数错误",
//   "request_id": "req-123", 
//   "data": {}
// }
```

### 自定义响应

```go
// 返回自定义响应
func RenderJson(ctx *gin.Context, code int, msg string, data interface{})
```

**示例:**
```go
r.GET("/custom", func(c *gin.Context) {
    render.RenderJson(c, 201, "Created successfully", gin.H{
        "id": 12345,
        "created_at": time.Now(),
    })
})
```

## SSE 流式响应

### 基本流式响应

```go
// 发送SSE事件
func RenderStream(ctx *gin.Context, id, event string, data interface{})
```

**示例:**
```go
r.GET("/stream", func(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    for i := 0; i < 10; i++ {
        render.RenderStream(c, fmt.Sprintf("msg-%d", i), "message", map[string]interface{}{
            "content": fmt.Sprintf("这是第 %d 条消息", i+1),
            "timestamp": time.Now(),
        })
        time.Sleep(time.Second)
    }
})
```

### 流式错误响应

```go
// 发送SSE错误事件
func RenderStreamFail(ctx *gin.Context, err error)
```

**示例:**
```go
r.GET("/stream-with-error", func(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    for i := 0; i < 5; i++ {
        if i == 3 {
            render.RenderStreamFail(c, errors.ErrorSystemError)
            return
        }
        render.RenderStream(c, fmt.Sprintf("msg-%d", i), "data", gin.H{
            "message": fmt.Sprintf("数据 %d", i),
        })
        time.Sleep(time.Second)
    }
})
```

## 错误堆栈追踪

```go
// 打印详细错误栈
func StackLogger(ctx *gin.Context, err error)
```

当错误包含堆栈信息时，系统会自动打印详细的错误堆栈：

```go
func someBusinessLogic() error {
    return errors.Wrap(someDeepError(), "业务逻辑失败")
}

r.GET("/business", func(c *gin.Context) {
    if err := someBusinessLogic(); err != nil {
        render.StackLogger(c, err) // 打印详细堆栈
        render.RenderJsonFail(c, err)
        return
    }
    render.RenderJsonSucc(c, gin.H{"status": "ok"})
})
```

## 响应头设置

所有响应都会自动设置以下头部信息：

- `code`: HTTP 状态码
- `message`: 响应消息
- `X-Request-ID`: 请求追踪ID

```go
// 响应头示例:
// code: 200
// message: success
// X-Request-ID: req-abc123
```

## 完整示例

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/render"
    "github.com/xiangtao94/golib/pkg/errors"
    "github.com/xiangtao94/golib/pkg/zlog"
)

func main() {
    r := gin.Default()
    
    // 添加请求ID中间件
    r.Use(func(c *gin.Context) {
        c.Set(zlog.ContextKeyRequestID, "req-"+fmt.Sprintf("%d", time.Now().UnixNano()))
        c.Next()
    })
    
    // API路由组
    api := r.Group("/api/v1")
    
    // 用户列表
    api.GET("/users", func(c *gin.Context) {
        users := []map[string]interface{}{
            {"id": 1, "name": "张三", "email": "zhangsan@example.com"},
            {"id": 2, "name": "李四", "email": "lisi@example.com"},
        }
        render.RenderJsonSucc(c, users)
    })
    
    // 用户详情
    api.GET("/users/:id", func(c *gin.Context) {
        userID := c.Param("id")
        
        // 模拟参数验证
        if userID == "" {
            render.RenderJsonFail(c, errors.ErrorParamInvalid)
            return
        }
        
        // 模拟用户不存在
        if userID == "999" {
            customErr := errors.NewError(1001, map[string]string{
                "zh": "用户不存在",
                "en": "User not found",
            })
            render.RenderJsonFail(c, customErr)
            return
        }
        
        user := map[string]interface{}{
            "id":    userID,
            "name":  "用户" + userID,
            "email": "user" + userID + "@example.com",
        }
        render.RenderJsonSucc(c, user)
    })
    
    // 实时消息流
    api.GET("/stream/messages", func(c *gin.Context) {
        c.Header("Content-Type", "text/event-stream")
        c.Header("Cache-Control", "no-cache")
        c.Header("Connection", "keep-alive")
        c.Header("Access-Control-Allow-Origin", "*")
        
        for i := 0; i < 10; i++ {
            // 模拟错误情况
            if i == 7 {
                render.RenderStreamFail(c, errors.ErrorSystemError)
                break
            }
            
            render.RenderStream(c, fmt.Sprintf("msg-%d", i), "message", map[string]interface{}{
                "id":      i + 1,
                "content": fmt.Sprintf("实时消息 #%d", i+1),
                "time":    time.Now().Format("15:04:05"),
            })
            
            time.Sleep(2 * time.Second)
        }
    })
    
    r.Run(":8080")
}
```

## 注意事项

- 所有响应都会自动添加请求ID，便于链路追踪
- 错误响应会自动根据语言设置返回对应的错误消息
- SSE 流式响应需要设置正确的响应头
- 错误堆栈只有在包含换行符时才会打印（避免简单错误的冗余输出）
- 自定义渲染器需要实现 `Render` 接口的所有方法 