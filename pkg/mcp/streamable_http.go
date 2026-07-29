package mcp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const sessionIDHeader = "Mcp-Session-Id"

// Register registers the streamable MCP HTTP transport on the Gin engine.
func (h *Handler) Register(r *gin.Engine) {
	streamable := officialmcp.NewStreamableHTTPHandler(
		func(*http.Request) *officialmcp.Server {
			return h.Server
		},
		&h.StreamableHTTPOpts,
	)
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(w, request.Body, h.MaxRequestBodyBytes)
		if h.ContextFn != nil {
			request = request.WithContext(h.ContextFn(request.Context(), request))
		}
		if h.isNewStatefulSession(request) {
			h.sessionAdmissionMu.Lock()
			defer h.sessionAdmissionMu.Unlock()
			if h.closed {
				http.Error(w, "MCP handler is closed", http.StatusServiceUnavailable)
				return
			}
			if activeSessionCount(h.Server) >= h.MaxActiveSessions {
				http.Error(
					w,
					"maximum active MCP sessions reached",
					http.StatusServiceUnavailable,
				)
				return
			}
		}
		streamable.ServeHTTP(w, request)
	})
	ginHandler := gin.WrapH(httpHandler)

	r.POST(h.BasePath, ginHandler)
	r.GET(h.BasePath, ginHandler)
	r.DELETE(h.BasePath, ginHandler)
}

func (h *Handler) isNewStatefulSession(request *http.Request) bool {
	return !h.StreamableHTTPOpts.Stateless &&
		request.Method == http.MethodPost &&
		request.Header.Get(sessionIDHeader) == ""
}

func activeSessionCount(server *officialmcp.Server) int {
	count := 0
	for range server.Sessions() {
		count++
	}
	return count
}
