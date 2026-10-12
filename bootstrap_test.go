package golib

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/health"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func bootstrapForTest(t *testing.T, engine *gin.Engine, name string, options ...BootstrapOption) *App {
	t.Helper()
	app, err := Bootstraps(engine, name, options...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Stop(context.Background())) })
	return app
}

func TestBootstrapsRecoversPanicsByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	observedStatus := 0
	engine.Use(func(ctx *gin.Context) {
		ctx.Next()
		observedStatus = ctx.Writer.Status()
	})
	bootstrapForTest(t, engine, "recovery-test",
		WithZlog(zlog.LogConfig{Stdout: false}),
	)
	engine.GET("/panic", func(*gin.Context) { panic("boom") })

	response := httptest.NewRecorder()
	require.NotPanics(t, func() {
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	})
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Equal(t, http.StatusInternalServerError, observedStatus)
}

func TestBootstrapsUsesCustomRecoveryOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	called := 0
	bootstrapForTest(t, engine, "custom-recovery-test",
		WithZlog(zlog.LogConfig{Stdout: false}),
		WithRecovery(func(ctx *gin.Context, _ any) {
			called++
			ctx.AbortWithStatus(http.StatusTeapot)
		}),
	)
	engine.GET("/panic", func(*gin.Context) { panic("boom") })

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	require.Equal(t, 1, called)
	require.Equal(t, http.StatusTeapot, response.Code)
}

func TestBootstrapsInitializesDefaultLoggerWithoutOption(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousAppName := env.GetAppName()
	t.Cleanup(func() {
		env.SetAppName(previousAppName)
	})
	_, err := zlog.InitLog(zlog.LogConfig{Level: "debug", Stdout: true})
	require.NoError(t, err)
	require.True(t, zlog.DebugEnabled(context.Background()))

	bootstrapForTest(t, gin.New(), "default-logger-test")

	require.Equal(t, "default-logger-test", env.GetAppName())
	require.False(t, zlog.DebugEnabled(context.Background()))
	require.True(t, zlog.InfoEnabled(context.Background()))
}

func TestBootstrapsUsesRequiredAppNameForLoggerAndMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousAppName := env.GetAppName()
	t.Cleanup(func() {
		env.SetAppName(previousAppName)
	})
	logDir := t.TempDir()
	engine := gin.New()
	logConfig := zlog.DefaultLogConfig()
	logConfig.LogToFile = true
	logConfig.LogDir = logDir
	bootstrapForTest(t, engine, "final-app",
		WithZlog(logConfig),
		WithPrometheus(),
	)
	engine.GET("/work", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/work", nil))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "final-app", env.GetAppName())
	require.Contains(t, response.Body.String(), `app_name="final-app"`)
	_, err := os.Stat(filepath.Join(logDir, "final-app.log"))
	require.NoError(t, err)
}

func TestBootstrapsMetricsObserveRecoveredPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	bootstrapForTest(t, engine, "metrics-test",
		WithZlog(zlog.LogConfig{Stdout: false}),
		WithPrometheus(),
	)
	engine.GET("/panic", func(*gin.Context) { panic("boom") })

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	require.Equal(t, http.StatusInternalServerError, response.Code)

	metrics := httptest.NewRecorder()
	engine.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Contains(t, metrics.Body.String(), `app_name="metrics-test",method="GET",route="/panic",status_class="5xx"`)
}

func TestBootstrapsRegistersLivenessByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	bootstrapForTest(t, engine, "liveness-test")

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/livez", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"status":"ok"}`, response.Body.String())

	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestWithReadinessUsesApplicationGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	gate := health.New()
	bootstrapForTest(t, engine, "readiness-test", WithReadiness(gate))

	request := func() int {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		return response.Code
	}

	require.Equal(t, http.StatusServiceUnavailable, request())
	gate.SetReady()
	require.Equal(t, http.StatusOK, request())
	gate.SetNotReady()
	require.Equal(t, http.StatusServiceUnavailable, request())
}

func TestWithReadinessRejectsNilGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, err := Bootstraps(gin.New(), "readiness-test", WithReadiness(nil))
	require.ErrorContains(t, err, "nil readiness gate")
}

func TestBootstrapsRequiresAppName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, err := Bootstraps(gin.New(), " ")
	require.ErrorContains(t, err, "app name is required")
	_, err = Bootstraps(gin.New(), "../invalid")
	require.ErrorContains(t, err, "invalid app name")
}

func TestBootstrapsDoesNotRegisterPprofByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	bootstrapForTest(t, engine, "pprof-test")

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

func TestHTTPServerWriteTimeoutCanBeDisabledForStreams(t *testing.T) {
	conf := DefaultHTTPServerConfig(8080)
	conf.WriteTimeout = NoWriteTimeout
	server := NewHTTPServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), conf)

	require.Zero(t, server.server.WriteTimeout)
	require.Equal(t, 30*time.Second, NewHTTPServer(http.NotFoundHandler(), HTTPServerConfig{}).server.WriteTimeout)
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
