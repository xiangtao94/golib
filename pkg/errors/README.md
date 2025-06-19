# 多语言错误处理

提供支持多语言的统一错误处理机制，集成 Gin 框架的国际化功能。

## 功能特性

- ✅ **多语言支持**: 支持中文、英文等多种语言的错误消息
- ✅ **统一错误码**: 预定义常用的业务错误码
- ✅ **框架集成**: 无缝集成 Gin 框架的国际化上下文
- ✅ **格式化支持**: 支持 sprintf 风格的错误消息格式化
- ✅ **预定义错误**: 提供常用的标准错误实例

## 快速开始

### 1. 基本使用

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/errors"
    "github.com/xiangtao94/golib/pkg/render"
)

func main() {
    r := gin.Default()
    
    r.GET("/test", func(c *gin.Context) {
        // 使用预定义错误
        err := errors.ErrorParamInvalid
        render.RenderJsonFail(c, err)
    })
    
    r.Run(":8080")
}
```

### 2. 自定义错误

```go
// 创建自定义错误
customErr := errors.NewError(1001, map[string]string{
    "zh": "用户不存在",
    "en": "User not found",
})

// 使用格式化错误
formattedErr := errors.ErrorCustomError.Sprintf("数据库连接失败")
```

## 预定义错误码

| 错误码 | 常量名 | 中文消息 | 英文消息 |
|--------|--------|----------|----------|
| 1 | SYSTEM_ERROR | 服务异常，请稍后重试 | Service exception, please try again later |
| 2 | PARAM_ERROR | 请求参数错误 | Request parameter error |
| 3 | USER_NOT_LOGIN | 用户Session已失效，请重新登录 | User session expired, please log in again |
| 4 | INVALID_REQUEST | 请求无效，请稍后再试 | Invalid request, please try again later |
| 100 | DEFAULT_ERROR | 服务开小差了，请稍后再试 | The service is down, please try again later |
| 101 | CUSTOM_ERROR | %s | %s |

## 预定义错误实例

```go
// 直接使用预定义的错误实例
var (
    ErrorParamInvalid   = NewError(PARAM_ERROR, nil)
    ErrorSystemError    = NewError(SYSTEM_ERROR, nil)
    ErrorUserNotLogin   = NewError(USER_NOT_LOGIN, nil)
    ErrorInvalidRequest = NewError(INVALID_REQUEST, nil)
    ErrorDefault        = NewError(DEFAULT_ERROR, nil)
    ErrorCustomError    = NewError(CUSTOM_ERROR, map[string]string{"zh": "%s", "en": "%s"})
)
```

## 使用示例

### 在业务逻辑中使用

```go
func validateUser(ctx *gin.Context, userID string) error {
    if userID == "" {
        return errors.ErrorParamInvalid
    }
    
    user := getUserFromDB(userID)
    if user == nil {
        // 创建自定义错误
        return errors.NewError(1001, map[string]string{
            "zh": "用户不存在",
            "en": "User not found",
        })
    }
    
    if !user.IsActive {
        // 使用格式化错误
        return errors.ErrorCustomError.Sprintf("用户 %s 已被禁用", user.Name)
    }
    
    return nil
}
```

### 在HTTP处理器中使用

```go
func userHandler(c *gin.Context) {
    userID := c.Param("id")
    
    if err := validateUser(c, userID); err != nil {
        // render.RenderJsonFail 会自动根据语言设置返回对应的错误消息
        render.RenderJsonFail(c, err)
        return
    }
    
    // 处理业务逻辑...
    render.RenderJsonSucc(c, gin.H{"message": "success"})
}
```

### 错误消息的语言选择

错误消息的语言选择基于以下优先级：

1. **请求上下文中的语言设置**: 从 `env.I18N_CONTEXT` 获取
2. **全局默认语言**: 通过 `env.GetLanguage()` 获取
3. **fallback**: 如果都没有，使用 "Unknown error"

## API 参考

### NewError

```go
func NewError(code int, messages map[string]string) Error
```

创建新的错误对象。如果 `messages` 为 nil，会自动从 `ErrMsg` 中获取对应语言的默认消息。

### Error.Sprintf

```go
func (err Error) Sprintf(v ...interface{}) Error
```

格式化错误消息，类似于 `fmt.Sprintf`。

### Error.GetMessage

```go
func (err Error) GetMessage(ctx *gin.Context) string
```

根据请求上下文获取对应语言的错误消息。

### Error.Error

```go
func (err Error) Error() string
```

实现标准库的 error 接口，返回当前默认语言的错误消息。 