// Package algo -----------------------------
// @file      : client_test.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/5/24 16:25
// Description:
// -------------------------------------------
package http

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func init() {
	gin.SetMode(gin.TestMode)
	zlog.InitLog(zlog.LogConfig{})
}

// mockHandler 用于模拟服务端处理
func mockHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ok":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"msg":"success"}`))
	case "/echo":
		w.Header().Set("Content-Type", "application/json")
		defer r.Body.Close()
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestClient_Get_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockHandler))
	defer server.Close()

	client := &ClientConf{
		Service:        "test",
		Domain:         server.URL,
		Timeout:        2 * time.Second,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = req
	opts := RequestOptions{
		Path: "/ok",
	}

	resp, err := client.Get(ctx, opts)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.HttpCode)
	assert.Equal(t, "{\"msg\":\"success\"}", string(resp.Response))
}

type TResult struct {
	Msg string `json:"msg"`
}

func TestClient_Post_Echo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockHandler))
	defer server.Close()

	client := &ClientConf{
		Service:        "test",
		Domain:         server.URL,
		Timeout:        2 * time.Second,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)

	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = req
	body := map[string]string{"key": "value"}
	opts := RequestOptions{
		Path:        "/echo",
		Encode:      EncodeJson,
		RequestBody: body,
	}

	resp, err := client.Post(ctx, opts)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.HttpCode)
}

func TestClient_Timeout(t *testing.T) {
	// 模拟一个超时服务
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(4 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &ClientConf{
		Service:        "test",
		Domain:         server.URL,
		Timeout:        3000 * time.Millisecond,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
		RetryTimes:     1,
	}

	ctx, _ := gin.CreateTestContext(nil)

	opts := RequestOptions{
		Path: "/timeout",
	}

	resp, err := client.Get(ctx, opts)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSSEStream(t *testing.T) {
	// 模拟一个 SSE 服务端
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Server does not support streaming")
		}

		events := []string{
			"data: hello world\n",
			"data: this is line 2\n",
			"data: final line\n",
		}

		for _, event := range events {
			fmt.Fprint(w, event)
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
		// 关闭连接模拟 EOF
	}))

	defer ts.Close()

	client := &ClientConf{
		Service:        "test",
		Domain:         ts.URL,
		Timeout:        2 * time.Second,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = req

	// 客户端发起请求
	resp, err := client.GetStream(ctx, RequestOptions{}, func(data []byte) error {
		fmt.Println(string(data))
		return nil
	})
	if err != nil {
		return
	}
	fmt.Println(string(resp.Response))
}
