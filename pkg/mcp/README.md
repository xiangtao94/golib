# MCP Gin 处理器

`pkg/mcp` 将官方 [`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk)
的 Streamable HTTP 服务接入 Gin。

## 基本用法

```go
package main

import (
	"context"

	"github.com/gin-gonic/gin"
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	ginmcp "github.com/xiangtao94/golib/pkg/mcp"
)

func main() {
	handler := ginmcp.NewHandler("example", "1.0.0")
	handler.Server.AddTool(
		&officialmcp.Tool{
			Name:        "hello",
			Description: "Say hello",
			InputSchema: map[string]any{"type": "object"},
		},
		func(context.Context, *officialmcp.CallToolRequest) (*officialmcp.CallToolResult, error) {
			return &officialmcp.CallToolResult{
				Content: []officialmcp.Content{
					&officialmcp.TextContent{Text: "hello"},
				},
			}, nil
		},
	)

	engine := gin.Default()
	handler.Register(engine)
	engine.Run(":8080")
}
```

`Handler.Server` 是官方 SDK server。工具处理器负责解析和校验
`request.Params.Arguments`。需要类型推导和自动 JSON Schema 校验时，直接使用官方泛型 API：

```go
officialmcp.AddTool(handler.Server, tool, typedHandler)
```

## 请求上下文

`WithContextFunc` 在请求进入 MCP transport 前扩展 `context.Context`，写入的值会传到工具处理器：

```go
handler := ginmcp.NewHandler(
	"example",
	"1.0.0",
	ginmcp.WithContextFunc(func(ctx context.Context, request *http.Request) context.Context {
		return context.WithValue(ctx, userContextKey, authenticate(request))
	}),
)
```

## 配置

```go
handler := ginmcp.NewHandler(
	"example",
	"1.0.0",
	ginmcp.WithBasePath("/api/mcp"),
	ginmcp.WithServerOptions(&officialmcp.ServerOptions{
		Instructions: "Use tools only when needed.",
	}),
	ginmcp.WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
		SessionTimeout: 30 * time.Minute,
	}),
)
```

服务在配置路径注册 `POST`、`GET` 和 `DELETE`，具体状态码、会话和流式行为由官方
Streamable HTTP transport 实现。

官方 SDK 没有旧实现的“会话级动态工具”和“按会话 ID 广播任意通知”抽象。本封装不模拟这些
非标准能力；需要服务端到客户端交互时，应通过官方 `ServerSession` API 和 MCP 标准消息实现。
