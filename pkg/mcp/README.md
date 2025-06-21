# MCP 服务器处理器

基于 `mark3labs/mcp-go` 封装的 Model Context Protocol (MCP) 服务器处理器，提供工具管理和会话控制功能。

## 功能特性

- ✅ **工具管理**: 支持添加、删除和管理 MCP 工具
- ✅ **会话控制**: 支持基于会话的工具管理
- ✅ **通知系统**: 支持向客户端发送通知消息
- ✅ **配置灵活**: 支持自定义基础路径和服务器选项
- ✅ **流式支持**: 支持 Server-Sent Events (SSE) 流式通信

## 快速开始

### 1. 基本使用

```go
package main

import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "github.com/xiangtao94/golib/pkg/mcp"
)

func main() {
    // 创建MCP处理器
    handler := mcp.NewHandler("my-app", "1.0.0")
    
    // 定义工具
    listTool := mcp.Tool{
        Name:        "list_files",
        Description: "List files in directory",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "path": map[string]interface{}{
                    "type":        "string",
                    "description": "Directory path",
                },
            },
            Required: []string{"path"},
        },
    }
    
    // 添加工具处理器
    handler.AddTool(listTool, func(args map[string]interface{}) (*server.CallToolResult, error) {
        path := args["path"].(string)
        // 实现文件列表逻辑
        return &server.CallToolResult{
            Content: []interface{}{
                map[string]interface{}{
                    "type": "text",
                    "text": fmt.Sprintf("Files in %s: file1.txt, file2.txt", path),
                },
            },
        }, nil
    })
}
```

### 2. 高级配置

```go
// 使用配置选项创建处理器
handler := mcp.NewHandler("advanced-app", "2.0.0",
    mcp.WithBasePath("/api/mcp"),
    mcp.WithContextFunc(func(r *http.Request) context.Context {
        return context.WithValue(r.Context(), "user", getUserFromRequest(r))
    }),
    mcp.WithServerOptions(
        server.WithMaxConnections(100),
    ),
    mcp.WitStreamableHTTPOptions(
        server.WithBufferSize(8192),
    ),
)
```

## 工具管理

### 添加全局工具

```go
// 方式1：单个工具
handler.AddTool(tool, handlerFunc)

// 方式2：批量添加
tools := []server.ServerTool{
    {
        Tool: calculatorTool,
        Handler: func(args map[string]interface{}) (*server.CallToolResult, error) {
            // 计算器逻辑
            return result, nil
        },
    },
    {
        Tool: weatherTool, 
        Handler: weatherHandler,
    },
}
handler.AddTools(tools...)
```

### 会话级工具管理

```go
// 为特定会话添加工具
err := handler.AddSessionTool("session-123", userTool, userHandler)
if err != nil {
    log.Fatal(err)
}

// 批量添加会话工具
sessionTools := []server.ServerTool{
    {Tool: privateTool, Handler: privateHandler},
}
err = handler.AddSessionTools("session-123", sessionTools...)

// 删除会话工具
err = handler.DeleteSessionTools("session-123", "tool1", "tool2")
```

## 通知系统

### 全局通知

```go
// 向所有客户端发送通知
handler.SendNotificationToAllClients("system_update", map[string]any{
    "version": "2.1.0",
    "message": "系统已更新到新版本",
})
```

### 特定客户端通知

```go
// 向特定客户端发送通知
err := handler.SendNotificationToSpecificClient("session-123", "user_message", map[string]any{
    "from": "admin",
    "text": "欢迎使用系统",
})
if err != nil {
    log.Printf("发送通知失败: %v", err)
}
```

## 配置选项

### WithBasePath

```go
// 设置MCP服务的基础路径
handler := mcp.NewHandler("app", "1.0.0", mcp.WithBasePath("/custom/mcp"))
```

### WithContextFunc

```go
// 自定义HTTP上下文处理
handler := mcp.NewHandler("app", "1.0.0", 
    mcp.WithContextFunc(func(r *http.Request) context.Context {
        // 添加用户认证信息到上下文
        token := r.Header.Get("Authorization")
        user := authenticateUser(token)
        return context.WithValue(r.Context(), "user", user)
    }),
)
```

### WithServerOptions

```go
// 添加MCP服务器配置选项
handler := mcp.NewHandler("app", "1.0.0",
    mcp.WithServerOptions(
        server.WithTimeout(30*time.Second),
        server.WithMaxConnections(200),
    ),
)
```

## 完整示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "github.com/xiangtao94/golib/pkg/mcp"
)

func main() {
    // 创建处理器
    handler := mcp.NewHandler("file-manager", "1.0.0",
        mcp.WithBasePath("/mcp"),
    )
    
    // 文件列表工具
    listTool := mcp.Tool{
        Name:        "list_files",
        Description: "List files in a directory",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "path": map[string]interface{}{
                    "type":        "string", 
                    "description": "Directory path to list",
                },
            },
            Required: []string{"path"},
        },
    }
    
    handler.AddTool(listTool, func(args map[string]interface{}) (*server.CallToolResult, error) {
        path := args["path"].(string)
        
        // 模拟文件列表
        files := []string{"document.txt", "image.jpg", "config.json"}
        
        return &server.CallToolResult{
            Content: []interface{}{
                map[string]interface{}{
                    "type": "text",
                    "text": fmt.Sprintf("Files in %s:\n- %s", path, 
                            fmt.Join(files, "\n- ")),
                },
            },
        }, nil
    })
    
    // 文件读取工具
    readTool := mcp.Tool{
        Name:        "read_file",
        Description: "Read content of a file",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "filepath": map[string]interface{}{
                    "type":        "string",
                    "description": "Path to the file to read",
                },
            },
            Required: []string{"filepath"},
        },
    }
    
    handler.AddTool(readTool, func(args map[string]interface{}) (*server.CallToolResult, error) {
        filepath := args["filepath"].(string)
        
        // 模拟文件读取
        content := fmt.Sprintf("Content of %s:\nHello, World!", filepath)
        
        return &server.CallToolResult{
            Content: []interface{}{
                map[string]interface{}{
                    "type": "text", 
                    "text": content,
                },
            },
        }, nil
    })
    
    // 获取底层服务器进行HTTP集成
    mcpServer := handler.GetServer()
    
    // 集成到HTTP服务器
    http.HandleFunc("/mcp/", func(w http.ResponseWriter, r *http.Request) {
        // 这里需要根据实际的mcp-go库API进行集成
        // 具体实现取决于库的HTTP处理方式
    })
    
    log.Println("MCP服务器启动在 :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## API 参考

### NewHandler

```go
func NewHandler(name, version string, opts ...MCPHandlerOption) *Handler
```

创建新的MCP处理器实例。

### 工具管理方法

- `AddTool(tool mcp.Tool, handler server.ToolHandlerFunc)`: 添加全局工具
- `AddTools(tools ...server.ServerTool)`: 批量添加全局工具
- `AddSessionTool(sessionID string, tool mcp.Tool, handler server.ToolHandlerFunc) error`: 添加会话工具
- `AddSessionTools(sessionID string, tools ...server.ServerTool) error`: 批量添加会话工具
- `DeleteSessionTools(sessionID string, names ...string) error`: 删除会话工具

### 通知方法

- `SendNotificationToAllClients(method string, params map[string]any)`: 全局通知
- `SendNotificationToSpecificClient(sessionID string, method string, params map[string]any) error`: 特定客户端通知

## 注意事项

- MCP工具需要定义完整的输入模式(InputSchema)
- 会话级工具优先级高于全局工具
- 通知系统支持实时向客户端推送消息
- 需要根据实际的mcp-go库版本调整集成方式 