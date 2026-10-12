package agui

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type trackingTransport struct{ closeCalls int }

func (*trackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unused transport")
}

func (transport *trackingTransport) CloseIdleConnections() { transport.closeCalls++ }

func TestGatewayClosesOnlyOwnedTransport(t *testing.T) {
	closed := make(chan struct{}, 1)
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	upstream.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			select {
			case closed <- struct{}{}:
			default:
			}
		}
	}
	upstream.Start()
	t.Cleanup(upstream.Close)
	gateway, err := New(Config{Endpoint: "https://agent.internal/run"})
	require.NoError(t, err)
	response, err := gateway.client.Get(upstream.URL)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	gateway.Close()
	gateway.Close()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("owned transport kept its idle connection open")
	}

	injected := &trackingTransport{}
	custom, err := New(Config{
		Endpoint: "https://agent.internal/run",
		Client:   &http.Client{Transport: injected},
	})
	require.NoError(t, err)
	custom.Close()
	require.Zero(t, injected.closeCalls)
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadline time.Time
	calls    int
}

func (recorder *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	recorder.deadline = deadline
	recorder.calls++
	return nil
}

func newTestGateway(t *testing.T, upstream roundTripFunc, modify func(*Config)) *Gateway {
	t.Helper()
	config := Config{
		Endpoint: "https://agent.internal/run",
		Client:   &http.Client{Transport: upstream},
		Headers: func(context.Context) (http.Header, error) {
			return http.Header{"Authorization": {"Bearer service-token"}}, nil
		},
	}
	if modify != nil {
		modify(&config)
	}
	gateway, err := New(config)
	require.NoError(t, err)
	return gateway
}

func testEngine(gateway *Gateway) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.GzipMiddleware())
	gateway.Register(engine, "/agent")
	engine.GET("/json", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return engine
}

func requestForRun(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set(zlog.HeaderRequestID, "req-123")
	request.Header.Set("Authorization", "Bearer browser-token")
	return request
}

func streamResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGatewayForwardsAGUIFramesAndOnlyClearsStreamDeadline(t *testing.T) {
	const input = `{"threadId":"thread-1","runId":"run-2","messages":[]}`
	const stream = ": keep-alive\n\n" +
		"data: {\"type\":\"RUN_STARTED\",\"threadId\":\"thread-1\",\"runId\":\"replay\"}\n\n" +
		"data: {\"type\":\"RUN_FINISHED\",\"threadId\":\"thread-1\",\"runId\":\"replay\"}\n\n" +
		"data: {\"type\":\"RUN_STARTED\",\"threadId\":\"thread-1\",\"runId\":\"run-2\"}\n\n" +
		"data: {\"type\":\"TEXT_MESSAGE_CONTENT\",\"messageId\":\"m1\",\"delta\":\"hello\"}\n\n" +
		"data: {\"type\":\"RUN_FINISHED\",\"threadId\":\"thread-1\",\"runId\":\"run-2\"}\n\n"
	gateway := newTestGateway(t, func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.Equal(t, input, string(body))
		require.Equal(t, "POST", request.Method)
		require.Equal(t, "https://agent.internal/run", request.URL.String())
		require.Equal(t, "text/event-stream", request.Header.Get("Accept"))
		require.Equal(t, "application/json", request.Header.Get("Content-Type"))
		require.Equal(t, "Bearer service-token", request.Header.Get("Authorization"))
		require.Equal(t, "", request.Header.Get("Cookie"))
		require.Equal(t, "req-123", request.Header.Get(zlog.HeaderRequestID))
		return streamResponse(http.StatusOK, stream), nil
	}, nil)
	engine := testEngine(gateway)
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder(), deadline: time.Now().Add(time.Minute)}
	Wrap(engine).ServeHTTP(response, requestForRun(input))

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, 1, response.calls)
	require.True(t, response.deadline.IsZero())
	require.Equal(t, "text/event-stream", response.Header().Get("Content-Type"))
	require.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
	require.Equal(t, "no", response.Header().Get("X-Accel-Buffering"))
	require.Equal(t, "req-123", response.Header().Get(zlog.HeaderRequestID))
	require.Empty(t, response.Header().Get("Content-Encoding"))
	require.Equal(t, stream, response.Body.String())

	regular := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(engine).ServeHTTP(regular, httptest.NewRequest(http.MethodGet, "/json", nil))
	require.Zero(t, regular.calls)
	require.Equal(t, http.StatusOK, regular.Code)
}

func TestGatewayPreservesPreStreamHTTPError(t *testing.T) {
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		return streamResponse(http.StatusUnprocessableEntity, ""), nil
	}, nil)
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(testEngine(gateway)).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, http.StatusUnprocessableEntity, response.Code)
	require.NotEqual(t, "text/event-stream", response.Header().Get("Content-Type"))
}

func TestGatewayNeverFollowsAgentRedirects(t *testing.T) {
	calls := 0
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": {"https://elsewhere.example/run"}},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}, nil)
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(testEngine(gateway)).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusTemporaryRedirect, response.Code)
	require.Empty(t, response.Header().Get("Location"))
}

func TestGatewayTurnsTruncatedRunIntoAGUIRunError(t *testing.T) {
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		return streamResponse(http.StatusOK,
			"data: {\"type\":\"RUN_STARTED\",\"threadId\":\"t\",\"runId\":\"r\"}\n\n"), nil
	}, nil)
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(testEngine(gateway)).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"type":"RUN_ERROR"`)
	require.Contains(t, response.Body.String(), `"code":"UPSTREAM_STREAM_ERROR"`)
}

func TestGatewayReportsTruncationBeforeTargetRunStarts(t *testing.T) {
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		return streamResponse(http.StatusOK, ": keep-alive\n\n"), nil
	}, nil)
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(testEngine(gateway)).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"type":"RUN_ERROR"`)
}

func TestGatewayAcceptsRunErrorBeforeRunStarted(t *testing.T) {
	const stream = "data: {\"type\":\"RUN_ERROR\",\"message\":\"Rejected\"}\n\n"
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		return streamResponse(http.StatusOK, stream), nil
	}, nil)
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(testEngine(gateway)).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, stream, response.Body.String())
}

func TestGatewayRejectsOversizedRequestBeforeCallingAgent(t *testing.T) {
	called := false
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("unexpected call")
	}, func(config *Config) {
		config.MaxRequestBytes = 10
	})
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(testEngine(gateway)).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.False(t, called)
}

func TestGatewayRequiresWrappedServerBeforeCallingAgent(t *testing.T) {
	called := false
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("unexpected call")
	}, nil)
	response := httptest.NewRecorder()
	testEngine(gateway).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.False(t, called)
}

func TestGatewayRejectsUnsupportedWriterBeforeCallingAgent(t *testing.T) {
	called := false
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("unexpected call")
	}, nil)
	response := httptest.NewRecorder()
	Wrap(testEngine(gateway)).ServeHTTP(response, requestForRun(`{"threadId":"t","runId":"r","messages":[]}`))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.False(t, called)
}

func TestReadFrameRejectsUnboundedEvent(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("data: " + strings.Repeat("x", 100) + "\n\n"))
	_, err := readFrame(reader, 32)
	require.ErrorIs(t, err, errEventTooLarge)
}

func TestGatewayRejectsDisabledSSEAccept(t *testing.T) {
	calls := 0
	gateway := newTestGateway(t, func(*http.Request) (*http.Response, error) {
		calls++
		return streamResponse(http.StatusOK, ""), nil
	}, nil)
	request := requestForRun(`{"threadId":"t","runId":"r","messages":[]}`)
	request.Header.Set("Accept", "text/event-stream; q=0")
	response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	Wrap(testEngine(gateway)).ServeHTTP(response, request)
	require.Equal(t, http.StatusNotAcceptable, response.Code)
	require.Zero(t, calls)
}
