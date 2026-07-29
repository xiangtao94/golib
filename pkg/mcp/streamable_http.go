package mcp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the streamable MCP HTTP transport on the Gin engine.
func (h *Handler) Register(r *gin.Engine) {
	streamable := officialmcp.NewStreamableHTTPHandler(
		func(*http.Request) *officialmcp.Server {
			return h.Server
		},
		&h.StreamableHTTPOpts,
	)
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if h.ContextFn != nil {
			request = request.WithContext(h.ContextFn(request.Context(), request))
		}
		streamable.ServeHTTP(w, request)
	})
	ginHandler := gin.WrapH(httpHandler)

	r.POST(h.BasePath, ginHandler)
	r.GET(h.BasePath, ginHandler)
	r.DELETE(h.BasePath, ginHandler)
}
