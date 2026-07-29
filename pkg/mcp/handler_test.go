package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type contextKey string

func TestNewHandlerUsesFiniteSessionTimeoutByDefault(t *testing.T) {
	handler := NewHandler("session-timeout-test", "1.0.0")

	if handler.StreamableHTTPOpts.SessionTimeout <= 0 {
		t.Fatalf("SessionTimeout = %s, want a finite positive default", handler.StreamableHTTPOpts.SessionTimeout)
	}
	if handler.MaxActiveSessions <= 0 {
		t.Fatalf("MaxActiveSessions = %d, want a finite positive default", handler.MaxActiveSessions)
	}
}

func TestStreamableHTTPOptionsWithoutTimeoutUseFiniteDefault(t *testing.T) {
	handler := NewHandler(
		"configured-session-test",
		"1.0.0",
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
		}),
	)

	if handler.StreamableHTTPOpts.SessionTimeout != DefaultSessionTimeout {
		t.Fatalf("SessionTimeout = %s, want %s", handler.StreamableHTTPOpts.SessionTimeout, DefaultSessionTimeout)
	}
}

func TestWithUnlimitedSessionLifetimeExplicitlyPreservesZeroTimeout(t *testing.T) {
	handler := NewHandler(
		"unlimited-session-test",
		"1.0.0",
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
		}),
		WithUnlimitedSessionLifetime(),
	)

	if handler.StreamableHTTPOpts.SessionTimeout != 0 {
		t.Fatalf("SessionTimeout = %s, want an explicitly unlimited lifetime", handler.StreamableHTTPOpts.SessionTimeout)
	}
}

func TestRegisterRejectsOversizedRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		"body-limit-test",
		"1.0.0",
		WithMaxRequestBodyBytes(64),
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
			Stateless:    true,
		}),
	)
	engine := gin.New()
	handler.Register(engine)

	request := httptest.NewRequest(
		http.MethodPost,
		"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping","padding":"`+strings.Repeat("x", 128)+`"}`),
	)
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusBadRequest, response.Body)
	}
}

func TestHandlerCloseClosesActiveSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		"close-test",
		"1.0.0",
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
		}),
	)
	engine := gin.New()
	handler.Register(engine)

	response := serveInitialize(engine)

	if response.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body)
	}
	if got := serverSessionCount(handler.Server); got != 1 {
		t.Fatalf("active sessions = %d, want 1", got)
	}

	if err := handler.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for serverSessionCount(handler.Server) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := serverSessionCount(handler.Server); got != 0 {
		t.Fatalf("active sessions after Close = %d, want 0", got)
	}

	response = serveInitialize(engine)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"initialize after Close status = %d, want %d; body = %s",
			response.Code,
			http.StatusServiceUnavailable,
			response.Body,
		)
	}
	if got := serverSessionCount(handler.Server); got != 0 {
		t.Fatalf("active sessions after rejected initialize = %d, want 0", got)
	}
}

func TestHandlerCloseIsNotBlockedByARequestBodyRead(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		"nonblocking-close-test",
		"1.0.0",
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
		}),
	)
	engine := gin.New()
	handler.Register(engine)

	body := &blockingReadCloser{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	request := httptest.NewRequest(http.MethodPost, "/mcp", body)
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	requestReturned := make(chan struct{})
	go func() {
		engine.ServeHTTP(httptest.NewRecorder(), request)
		close(requestReturned)
	}()

	select {
	case <-body.started:
	case <-time.After(time.Second):
		t.Fatal("request body was not read")
	}

	closeReturned := make(chan error, 1)
	go func() {
		closeReturned <- handler.Close()
	}()
	select {
	case err := <-closeReturned:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		close(body.release)
		<-requestReturned
		t.Fatal("Close() was blocked by request body I/O")
	}

	close(body.release)
	select {
	case <-requestReturned:
	case <-time.After(time.Second):
		t.Fatal("request did not return after its body was released")
	}
	if got := serverSessionCount(handler.Server); got != 0 {
		t.Fatalf("active sessions after Close = %d, want 0", got)
	}
}

type blockingReadCloser struct {
	started     chan struct{}
	startedOnce sync.Once
	release     chan struct{}
}

func (body *blockingReadCloser) Read([]byte) (int, error) {
	body.startedOnce.Do(func() {
		close(body.started)
	})
	<-body.release
	return 0, io.EOF
}

func (*blockingReadCloser) Close() error {
	return nil
}

func TestRegisterRejectsSessionsAboveActiveLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		"session-limit-test",
		"1.0.0",
		WithMaxActiveSessions(2),
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
		}),
	)
	engine := gin.New()
	handler.Register(engine)
	t.Cleanup(func() {
		if err := handler.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	for index, wantStatus := range []int{
		http.StatusOK,
		http.StatusOK,
		http.StatusServiceUnavailable,
	} {
		response := serveInitialize(engine)

		if response.Code != wantStatus {
			t.Fatalf(
				"request %d status = %d, want %d; body = %s",
				index+1,
				response.Code,
				wantStatus,
				response.Body,
			)
		}
	}
	if got := serverSessionCount(handler.Server); got != 2 {
		t.Fatalf("active sessions = %d, want 2", got)
	}
}

func TestRegisterEnforcesActiveSessionLimitConcurrently(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		maxSessions = 2
		requests    = 20
	)
	handler := NewHandler(
		"concurrent-session-limit-test",
		"1.0.0",
		WithMaxActiveSessions(maxSessions),
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
		}),
	)
	engine := gin.New()
	handler.Register(engine)
	t.Cleanup(func() {
		if err := handler.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	start := make(chan struct{})
	statuses := make(chan int, requests)
	var wait sync.WaitGroup
	for range requests {
		wait.Go(func() {
			<-start
			statuses <- serveInitialize(engine).Code
		})
	}
	close(start)
	wait.Wait()
	close(statuses)

	accepted := 0
	rejected := 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			accepted++
		case http.StatusServiceUnavailable:
			rejected++
		default:
			t.Errorf("unexpected initialize status = %d", status)
		}
	}
	if accepted != maxSessions || rejected != requests-maxSessions {
		t.Fatalf(
			"accepted = %d, rejected = %d; want %d and %d",
			accepted,
			rejected,
			maxSessions,
			requests-maxSessions,
		)
	}
	if got := serverSessionCount(handler.Server); got != maxSessions {
		t.Fatalf("active sessions = %d, want %d", got, maxSessions)
	}
}

func serveInitialize(handler http.Handler) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost,
		"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`),
	)
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func serverSessionCount(server *officialmcp.Server) int {
	count := 0
	for range server.Sessions() {
		count++
	}
	return count
}

func TestRegisterPassesHTTPContextToToolHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const actorKey contextKey = "actor"
	handler := NewHandler(
		"context-test",
		"1.0.0",
		WithContextFunc(func(ctx context.Context, request *http.Request) context.Context {
			return context.WithValue(ctx, actorKey, request.Header.Get("X-Actor"))
		}),
		WithStreamableHTTPOptions(&officialmcp.StreamableHTTPOptions{
			JSONResponse: true,
			Stateless:    true,
		}),
	)
	handler.Server.AddTool(
		&officialmcp.Tool{
			Name:        "context_value",
			Description: "returns a value injected from the HTTP request",
			InputSchema: map[string]any{"type": "object"},
		},
		func(ctx context.Context, _ *officialmcp.CallToolRequest) (*officialmcp.CallToolResult, error) {
			value, _ := ctx.Value(actorKey).(string)
			return &officialmcp.CallToolResult{
				Content: []officialmcp.Content{&officialmcp.TextContent{Text: value}},
			}, nil
		},
	)

	engine := gin.New()
	handler.Register(engine)

	request := httptest.NewRequest(
		http.MethodPost,
		"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"context_value","arguments":{}}}`),
	)
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "alice")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body)
	}
	var envelope struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body)
	}
	if len(envelope.Result.Content) != 1 || envelope.Result.Content[0].Text != "alice" {
		t.Fatalf("content = %#v, want injected actor alice", envelope.Result.Content)
	}
}
