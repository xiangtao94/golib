package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type contextKey string

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
	handler.AddTool(
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
