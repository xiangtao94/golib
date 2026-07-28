// Package algo -----------------------------
// @file      : client_test.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/5/24 16:25
// Description:
// -------------------------------------------
package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func TestMain(m *testing.M) {
	if _, err := zlog.InitLog(zlog.LogConfig{}); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = zlog.CloseLogger()
	os.Exit(code)
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

	client, err := NewClient(ClientConfig{
		Service:        "test",
		Domain:         server.URL,
		Timeout:        2 * time.Second,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
	})
	require.NoError(t, err)
	defer client.Close()
	opts := RequestOptions{
		Path: "/ok",
	}

	resp, err := client.Get(context.Background(), opts)

	require.NoError(t, err)
	require.Equal(t, 200, resp.HttpCode)
	require.Equal(t, "{\"msg\":\"success\"}", string(resp.Response))
}

type TResult struct {
	Msg string `json:"msg"`
}

func TestClient_Post_Echo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockHandler))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Service:        "test",
		Domain:         server.URL,
		Timeout:        2 * time.Second,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
	})
	require.NoError(t, err)
	defer client.Close()
	body := map[string]string{"key": "value"}
	opts := RequestOptions{
		Path:        "/echo",
		Encode:      EncodeJson,
		RequestBody: body,
	}

	resp, err := client.Post(context.Background(), opts)

	require.NoError(t, err)
	require.Equal(t, 200, resp.HttpCode)
}

func TestClient_Timeout(t *testing.T) {
	// 模拟一个超时服务
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Service:        "test",
		Domain:         server.URL,
		Timeout:        20 * time.Millisecond,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
		RetryCount:     0,
	})
	require.NoError(t, err)
	defer client.Close()

	opts := RequestOptions{
		Path: "/timeout",
	}

	resp, err := client.Get(context.Background(), opts)

	require.Error(t, err)
	require.Nil(t, resp)
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
			_, _ = w.Write([]byte(event))
			flusher.Flush()
		}
		// 关闭连接模拟 EOF
	}))

	defer ts.Close()

	client, err := NewClient(ClientConfig{
		Service:        "test",
		Domain:         ts.URL,
		Timeout:        2 * time.Second,
		MaxReqBodyLen:  1024,
		MaxRespBodyLen: 1024,
	})
	require.NoError(t, err)
	defer client.Close()

	var lines []string
	resp, err := client.GetStream(context.Background(), RequestOptions{}, func(data []byte) error {
		lines = append(lines, string(data))
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.HttpCode)
	require.Equal(t, []string{
		"data: hello world",
		"data: this is line 2",
		"data: final line",
	}, lines)
}
