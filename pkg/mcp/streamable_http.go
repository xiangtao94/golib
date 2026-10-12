package mcp

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const sessionIDHeader = "Mcp-Session-Id"

// Register registers the streamable MCP HTTP transport on a Gin router or group.
func (h *Handler) Register(r gin.IRoutes) {
	opts := h.StreamableHTTPOpts
	if h.MaxRequestBodyBytes > 0 {
		opts.MaxRequestBodyBytes = h.MaxRequestBodyBytes
	}
	streamable := officialmcp.NewStreamableHTTPHandler(
		func(*http.Request) *officialmcp.Server {
			return h.Server
		},
		&opts,
	)
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if h.ContextFn != nil {
			request = request.WithContext(h.ContextFn(request.Context(), request))
		}
		if h.isNewStatefulSession(request) {
			admittedRequest, admissionID, rejection := h.admitSession(request)
			if rejection != "" {
				http.Error(w, rejection, http.StatusServiceUnavailable)
				return
			}
			request = admittedRequest
			defer func() {
				h.finishSessionAdmission(admissionID, w.Header().Get(sessionIDHeader))
			}()
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

func (h *Handler) admitSession(request *http.Request) (*http.Request, uint64, string) {
	for {
		h.sessionAdmissionMu.Lock()
		if h.closed {
			h.sessionAdmissionMu.Unlock()
			return request, 0, "MCP handler is closed"
		}
		revision := h.sessionAdmissionRevision
		h.sessionAdmissionMu.Unlock()

		// Sessions() copies the SDK's session list. Keep that O(n) work out of
		// the admission lock, then retry if an in-flight admission completed.
		activeSessions := activeSessionCount(h.Server)
		h.sessionAdmissionMu.Lock()
		if revision != h.sessionAdmissionRevision {
			h.sessionAdmissionMu.Unlock()
			continue
		}
		if h.closed {
			h.sessionAdmissionMu.Unlock()
			return request, 0, "MCP handler is closed"
		}
		pendingAdmissions := len(h.sessionAdmissions)
		if activeSessions+pendingAdmissions < h.MaxActiveSessions {
			ctx, cancel := context.WithCancel(request.Context())
			h.nextSessionAdmission++
			admissionID := h.nextSessionAdmission
			if h.sessionAdmissions == nil {
				h.sessionAdmissions = make(map[uint64]context.CancelFunc)
			}
			h.sessionAdmissions[admissionID] = cancel
			h.sessionAdmissionMu.Unlock()
			return request.WithContext(ctx), admissionID, ""
		}
		if pendingAdmissions == 0 {
			h.sessionAdmissionMu.Unlock()
			return request, 0, "maximum active MCP sessions reached"
		}
		if h.sessionAdmissionChange == nil {
			h.sessionAdmissionChange = make(chan struct{})
		}
		changed := h.sessionAdmissionChange
		h.sessionAdmissionMu.Unlock()

		select {
		case <-changed:
		case <-request.Context().Done():
			return request, 0, "MCP session initialization canceled"
		}
	}
}

func (h *Handler) finishSessionAdmission(admissionID uint64, sessionID string) {
	h.sessionAdmissionMu.Lock()
	cancel := h.sessionAdmissions[admissionID]
	delete(h.sessionAdmissions, admissionID)
	if cancel != nil {
		h.sessionAdmissionRevision++
		h.notifySessionAdmissionChangeLocked()
	}
	closed := h.closed
	h.sessionAdmissionMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if closed && sessionID != "" {
		closeSessionByID(h.Server, sessionID)
	}
}

func (h *Handler) notifySessionAdmissionChangeLocked() {
	if h.sessionAdmissionChange != nil {
		close(h.sessionAdmissionChange)
	}
	h.sessionAdmissionChange = make(chan struct{})
}

func closeSessionByID(server *officialmcp.Server, sessionID string) {
	for session := range server.Sessions() {
		if session.ID() == sessionID {
			_ = session.Close()
			return
		}
	}
}
