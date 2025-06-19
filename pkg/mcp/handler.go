// Package mcp -----------------------------
// @file      : types.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/6/16 02:16
// -------------------------------------------
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handler struct {
	server             *server.MCPServer
	BasePath           string
	ContextFn          server.HTTPContextFunc
	ServerOpts         []server.ServerOption
	StreamableHTTPOpts []server.StreamableHTTPOption
	BaseURL            string
}

// MCPHandlerOption 是配置MCPHandler的函数选项
type MCPHandlerOption func(*Handler)

func NewHandler(name, version string, opts ...MCPHandlerOption) *Handler {
	h := &Handler{
		BasePath:           "/mcp",
		ServerOpts:         []server.ServerOption{},
		StreamableHTTPOpts: []server.StreamableHTTPOption{},
	}
	for _, opt := range opts {
		opt(h)
	}
	// 创建MCP服务器
	h.server = server.NewMCPServer(name, version, h.ServerOpts...)
	return h
}

// GetServer 返回底层的MCP服务器实例
func (h *Handler) GetServer() *server.MCPServer {
	return h.server
}

// WithBasePath 设置MCP处理器的基础路径
func WithBasePath(path string) MCPHandlerOption {
	return func(h *Handler) {
		h.BasePath = path
	}
}

// WithContextFunc 设置HTTP上下文函数
func WithContextFunc(fn server.HTTPContextFunc) MCPHandlerOption {
	return func(h *Handler) {
		h.ContextFn = fn
	}
}

// WithServerOptions 添加MCP服务器选项
func WithServerOptions(opts ...server.ServerOption) MCPHandlerOption {
	return func(h *Handler) {
		h.ServerOpts = append(h.ServerOpts, opts...)
	}
}

// WithSSEOptions 添加SSE服务器选项
func WitStreamableHTTPOptions(opts ...server.StreamableHTTPOption) MCPHandlerOption {
	return func(h *Handler) {
		h.StreamableHTTPOpts = append(h.StreamableHTTPOpts, opts...)
	}
}

// AddTool 向MCP服务器添加工具
func (h *Handler) AddTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	h.server.AddTool(tool, handler)
}

// AddTools 向MCP服务器添加多个工具
func (h *Handler) AddTools(tools ...server.ServerTool) {
	h.server.AddTools(tools...)
}

// AddSessionTool 向特定会话添加工具
func (h *Handler) AddSessionTool(sessionID string, tool mcp.Tool, handler server.ToolHandlerFunc) error {
	return h.server.AddSessionTool(sessionID, tool, handler)
}

// AddSessionTools 向特定会话添加多个工具
func (h *Handler) AddSessionTools(sessionID string, tools ...server.ServerTool) error {
	return h.server.AddSessionTools(sessionID, tools...)
}

// DeleteSessionTools 从特定会话删除工具
func (h *Handler) DeleteSessionTools(sessionID string, names ...string) error {
	return h.server.DeleteSessionTools(sessionID, names...)
}

// SendNotificationToAllClients 向所有客户端发送通知
func (h *Handler) SendNotificationToAllClients(method string, params map[string]any) {
	h.server.SendNotificationToAllClients(method, params)
}

// SendNotificationToSpecificClient 向特定客户端发送通知
func (h *Handler) SendNotificationToSpecificClient(sessionID string, method string, params map[string]any) error {
	return h.server.SendNotificationToSpecificClient(sessionID, method, params)
}
