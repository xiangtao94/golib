// Package mcp provides a Gin adapter for the official Model Context Protocol Go SDK.
package mcp

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// DefaultSessionTimeout bounds how long an idle stateful MCP session is
	// retained when no streamable HTTP options are supplied.
	DefaultSessionTimeout = 30 * time.Minute
	// DefaultMaxRequestBodyBytes bounds memory consumed while the upstream MCP
	// transport buffers a request body.
	DefaultMaxRequestBodyBytes int64 = 4 << 20
	// DefaultMaxActiveSessions bounds stateful MCP session fan-out.
	DefaultMaxActiveSessions = 1_024
)

// HTTPContextFunc enriches the request context before the MCP transport handles
// the request. Values added here are available to tool handlers.
type HTTPContextFunc func(context.Context, *http.Request) context.Context

type Handler struct {
	Server              *officialmcp.Server
	BasePath            string
	ContextFn           HTTPContextFunc
	ServerOpts          officialmcp.ServerOptions
	StreamableHTTPOpts  officialmcp.StreamableHTTPOptions
	MaxRequestBodyBytes int64
	MaxActiveSessions   int

	allowUnlimitedSessions bool
	sessionAdmissionMu     sync.Mutex
	closed                 bool
}

// MCPHandlerOption configures a Handler before its MCP server is created.
type MCPHandlerOption func(*Handler)

func NewHandler(name, version string, opts ...MCPHandlerOption) *Handler {
	h := &Handler{
		BasePath:            "/mcp",
		MaxRequestBodyBytes: DefaultMaxRequestBodyBytes,
		MaxActiveSessions:   DefaultMaxActiveSessions,
	}
	for _, opt := range opts {
		opt(h)
	}
	if !h.StreamableHTTPOpts.Stateless &&
		h.StreamableHTTPOpts.SessionTimeout <= 0 &&
		!h.allowUnlimitedSessions {
		h.StreamableHTTPOpts.SessionTimeout = DefaultSessionTimeout
	}
	if h.MaxRequestBodyBytes <= 0 {
		h.MaxRequestBodyBytes = DefaultMaxRequestBodyBytes
	}
	if h.MaxActiveSessions <= 0 {
		h.MaxActiveSessions = DefaultMaxActiveSessions
	}
	h.Server = officialmcp.NewServer(
		&officialmcp.Implementation{Name: name, Version: version},
		&h.ServerOpts,
	)
	return h
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

// WithUnlimitedSessionLifetime disables idle session expiry for stateful MCP
// transports. Prefer the finite default unless another lifecycle owner closes
// every session.
func WithUnlimitedSessionLifetime() MCPHandlerOption {
	return func(h *Handler) {
		h.StreamableHTTPOpts.SessionTimeout = 0
		h.allowUnlimitedSessions = true
	}
}

// WithMaxRequestBodyBytes sets the maximum MCP request body size. Non-positive
// values use [DefaultMaxRequestBodyBytes].
func WithMaxRequestBodyBytes(maxBytes int64) MCPHandlerOption {
	return func(h *Handler) {
		h.MaxRequestBodyBytes = maxBytes
	}
}

// WithMaxActiveSessions sets the maximum number of stateful MCP sessions.
// Non-positive values use [DefaultMaxActiveSessions].
func WithMaxActiveSessions(maxSessions int) MCPHandlerOption {
	return func(h *Handler) {
		h.MaxActiveSessions = maxSessions
	}
}

// Close permanently stops accepting new stateful MCP sessions and gracefully
// closes all sessions currently connected to the server. It is safe to call
// Close more than once.
func (h *Handler) Close() error {
	if h == nil || h.Server == nil {
		return nil
	}

	h.sessionAdmissionMu.Lock()
	defer h.sessionAdmissionMu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true

	var errs []error
	for session := range h.Server.Sessions() {
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
