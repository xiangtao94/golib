package golib

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/xiangtao94/golib/pkg/zlog"
)

func TestAppRegistersRoutesAndClosesResourcesOnRun(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, err := NewApp("app-test", WithZlog(zlog.LogConfig{Stdout: false}))
	require.NoError(t, err)
	app.GET("/work", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/work", nil))
	require.Equal(t, http.StatusNoContent, response.Code)

	closed := false
	require.NoError(t, app.AddStop("resource", func(context.Context) error {
		closed = true
		return nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, app.RunWithConfig(ctx, HTTPServerConfig{Addr: "127.0.0.1:0"}))
	require.True(t, closed)
}

func TestNewAppRejectsInvalidName(t *testing.T) {
	_, err := NewApp(" ")
	require.ErrorContains(t, err, "app name is required")
}

func TestAppStopsRegisteredResourcesBeforeLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logDir := t.TempDir()
	logConfig := zlog.DefaultLogConfig()
	logConfig.Stdout = false
	logConfig.LogToFile = true
	logConfig.LogDir = logDir
	app, err := NewApp("resource-test", WithZlog(logConfig))
	require.NoError(t, err)
	closed := 0
	require.NoError(t, app.AddStop("resource", func(context.Context) error {
		closed++
		zlog.Info(context.Background(), "resource closed")
		return nil
	}))

	require.NoError(t, app.Stop(context.Background()))
	require.NoError(t, app.Stop(context.Background()))
	require.Equal(t, 1, closed)
	logBytes, err := os.ReadFile(filepath.Join(logDir, "resource-test.log"))
	require.NoError(t, err)
	require.Contains(t, string(logBytes), "resource closed")
	require.Error(t, app.AddStop("late", func(context.Context) error { return nil }))
}

func TestAppRunServerClosesResourcesWhenServerFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, err := NewApp("run-failure-test", WithZlog(zlog.LogConfig{Stdout: false}))
	require.NoError(t, err)
	closed := false
	require.NoError(t, app.AddStop("resource", func(context.Context) error {
		closed = true
		return nil
	}))
	server := NewHTTPServer(http.NotFoundHandler(), HTTPServerConfig{Addr: "invalid-address"})

	err = app.RunServer(context.Background(), server)
	require.ErrorContains(t, err, "listen on invalid-address")
	require.True(t, closed)
}

func TestAppRunServerClosesResourcesAfterCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, err := NewApp("run-cancel-test", WithZlog(zlog.LogConfig{Stdout: false}))
	require.NoError(t, err)
	closed := false
	require.NoError(t, app.AddStop("resource", func(context.Context) error {
		closed = true
		return nil
	}))
	server := NewHTTPServer(app.Engine, HTTPServerConfig{Addr: "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, app.RunServer(ctx, server))
	require.True(t, closed)
}

func TestBootstrapsReturnsAppWithExistingEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	app, err := Bootstraps(engine, "existing-engine-test", WithZlog(zlog.LogConfig{Stdout: false}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Stop(context.Background())) })
	require.Same(t, engine, app.Engine)
}
