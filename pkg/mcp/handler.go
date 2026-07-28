// Package mcp provides a Gin adapter for the official Model Context Protocol Go SDK.
package mcp

import (
	"context"
	"net/http"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPContextFunc enriches the request context before the MCP transport handles
// the request. Values added here are available to tool handlers.
type HTTPContextFunc func(context.Context, *http.Request) context.Context

type Handler struct {
	server             *officialmcp.Server
	BasePath           string
	ContextFn          HTTPContextFunc
	ServerOpts         officialmcp.ServerOptions
	StreamableHTTPOpts officialmcp.StreamableHTTPOptions
}

// MCPHandlerOption configures a Handler before its MCP server is created.
type MCPHandlerOption func(*Handler)

func NewHandler(name, version string, opts ...MCPHandlerOption) *Handler {
	h := &Handler{BasePath: "/mcp"}
	for _, opt := range opts {
		opt(h)
	}
	h.server = officialmcp.NewServer(
		&officialmcp.Implementation{Name: name, Version: version},
		&h.ServerOpts,
	)
	return h
}

// GetServer returns the underlying official MCP server.
func (h *Handler) GetServer() *officialmcp.Server {
	return h.server
}

// WithBasePath sets the Gin route used by the streamable HTTP transport.
func WithBasePath(path string) MCPHandlerOption {
	return func(h *Handler) {
		h.BasePath = path
	}
}

// WithContextFunc sets the function used to enrich every MCP HTTP request context.
func WithContextFunc(fn HTTPContextFunc) MCPHandlerOption {
	return func(h *Handler) {
		h.ContextFn = fn
	}
}

// WithServerOptions configures the official MCP server.
func WithServerOptions(opts *officialmcp.ServerOptions) MCPHandlerOption {
	return func(h *Handler) {
		if opts != nil {
			h.ServerOpts = *opts
		}
	}
}

// WithStreamableHTTPOptions configures the official streamable HTTP transport.
func WithStreamableHTTPOptions(opts *officialmcp.StreamableHTTPOptions) MCPHandlerOption {
	return func(h *Handler) {
		if opts != nil {
			h.StreamableHTTPOpts = *opts
		}
	}
}

// AddTool adds or replaces a tool on the MCP server.
//
// This exposes the official SDK's low-level API. The caller is responsible for
// decoding and validating req.Params.Arguments. Callers wanting typed schema
// inference can use officialmcp.AddTool with GetServer.
func (h *Handler) AddTool(tool *officialmcp.Tool, handler officialmcp.ToolHandler) {
	h.server.AddTool(tool, handler)
}

// RemoveTools removes tools by name. Removing an unknown tool is a no-op.
func (h *Handler) RemoveTools(names ...string) {
	h.server.RemoveTools(names...)
}
