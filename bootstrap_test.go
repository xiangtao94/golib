package golib

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBootstrapsDoesNotRegisterPprofByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	Bootstraps(engine)

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))

	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestDefaultHTTPServerConfigHasDefensiveTimeouts(t *testing.T) {
	conf := DefaultHTTPServerConfig(8080)

	require.Equal(t, ":8080", conf.Addr)
	require.Positive(t, conf.ReadHeaderTimeout)
	require.Positive(t, conf.ReadTimeout)
	require.Positive(t, conf.WriteTimeout)
	require.Positive(t, conf.IdleTimeout)
	require.Positive(t, conf.ShutdownTimeout)
	require.Positive(t, conf.MaxHeaderBytes)
}

func TestHTTPServerServeStopsWhenContextIsCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := NewHTTPServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), DefaultHTTPServerConfig(0))

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- server.Serve(ctx, listener)
	}()

	response, err := http.Get("http://" + listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusNoContent, response.StatusCode)

	cancel()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("HTTP server did not stop after context cancellation")
	}
}

func TestHTTPServerServeRejectsNilContext(t *testing.T) {
	server := NewHTTPServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not be called")
	}), DefaultHTTPServerConfig(0))

	//lint:ignore SA1012 This test verifies that nil contexts are rejected.
	require.ErrorIs(t, server.Serve(nil, nil), ErrNilContext)
}

func TestHTTPServerRunRejectsNilContextBeforeListening(t *testing.T) {
	server := NewHTTPServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not be called")
	}), HTTPServerConfig{Addr: "invalid-address"})

	//lint:ignore SA1012 This test verifies that nil contexts are rejected.
	require.ErrorIs(t, server.Run(nil), ErrNilContext)
}

func TestRegisterPprofExplicitlyRegistersHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	RegisterPprof(engine)

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))

	require.Equal(t, http.StatusOK, response.Code)
}
