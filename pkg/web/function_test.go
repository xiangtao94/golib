package web_test

import (
	"context"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/require"
	serviceerrors "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/render"
	"github.com/xiangtao94/golib/pkg/web"
)

func TestFunctionBindingAndErrors(t *testing.T) {
	type request struct {
		Name string `json:"name" form:"name" binding:"required"`
	}
	for _, tc := range []struct {
		name, contentType, body string
		forceJSON               bool
		actionError             error
		status                  int
		called                  bool
	}{
		{"automatic JSON", "application/json", `{"name":"alice"}`, false, nil, 200, true},
		{"automatic form", "application/x-www-form-urlencoded", "name=alice", false, nil, 200, true},
		{"explicit JSON overrides form", "application/x-www-form-urlencoded", `{"name":"alice"}`, true, nil, 200, true},
		{"explicit JSON without header", "", `{"name":"alice"}`, true, nil, 200, true},
		{"malformed JSON", "application/json", `{`, false, nil, 400, false},
		{"validation failure", "application/json", `{}`, false, nil, 400, false},
		{"business error", "application/json", `{"name":"alice"}`, false, serviceerrors.ErrUnauthenticated, 401, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			var options []web.Option
			if tc.forceJSON {
				options = append(options, web.WithBinding(binding.JSON))
			}
			engine := gin.New()
			engine.POST("/", web.Handle(func(ctx *gin.Context, req *request) (any, error) {
				called = true
				require.Equal(t, "alice", req.Name)
				return req, tc.actionError
			}, options...))
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, tc.called, called)
			require.NotEmpty(t, response.Header().Get("X-Request-ID"))
		})
	}
}

func TestFunctionRendererAndFreshRequest(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}
	renderer := render.NewJSONRenderer(func(r render.Response) any { return gin.H{"payload": r.Data} })
	engine := gin.New()
	engine.POST("/", web.Handle(func(ctx *gin.Context, req *request) (any, error) { return req.Name, nil }, web.WithRenderer(renderer), web.WithSuccessStatus(http.StatusAccepted)))
	for _, tc := range []struct{ body, expected string }{{`{"name":"alice"}`, `{"payload":"alice"}`}, {`{}`, `{"payload":""}`}} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		require.Equal(t, http.StatusAccepted, response.Code)
		require.JSONEq(t, tc.expected, response.Body.String())
	}
}

func TestFunctionReceivesHTTPContext(t *testing.T) {
	type request struct {
		Name string `form:"name"`
	}
	engine := gin.New()
	engine.GET("/items/:id", web.Handle(func(c *gin.Context, req *request) (any, error) {
		require.Equal(t, "item-7", c.Param("id"))
		require.Equal(t, "zh-CN", c.GetHeader("Accept-Language"))
		require.Equal(t, "alice", req.Name)
		require.Equal(t, "http-context-test", zlog.GetRequestID(c.Request.Context()))
		require.ErrorIs(t, c.Request.Context().Err(), context.Canceled)
		return gin.H{"id": c.Param("id")}, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/items/item-7?name=alice", nil).WithContext(ctx)
	req.Header.Set("Accept-Language", "zh-CN")
	req.Header.Set("X-Request-ID", "http-context-test")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"id":"item-7"`)
}
