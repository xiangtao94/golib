package golib

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/lifecycle"
)

// App owns the Gin engine and resources initialized during bootstrap.
// Routes can be registered directly through the embedded Gin engine.
type App struct {
	*gin.Engine
	manager *lifecycle.Manager
}

// NewApp creates a Gin engine and applies the standard bootstrap defaults.
func NewApp(appName string, opts ...BootstrapOption) (*App, error) {
	return Bootstraps(gin.New(), appName, opts...)
}

// AddStop registers an already initialized application resource for shutdown.
func (app *App) AddStop(name string, stop func(context.Context) error) error {
	return app.manager.AddStop(name, stop)
}

// Stop closes all registered resources. It is safe to call more than once.
func (app *App) Stop(ctx context.Context) error {
	return app.manager.Stop(ctx)
}

// Run serves on the default port and closes registered resources on exit.
func (app *App) Run(ctx context.Context) error {
	return app.RunWithConfig(ctx, DefaultHTTPServerConfig(8080))
}

// RunWithConfig serves using the supplied HTTP settings and closes resources.
func (app *App) RunWithConfig(ctx context.Context, config HTTPServerConfig) error {
	return app.RunServer(ctx, NewHTTPServer(app.Engine, config))
}

// RunServer serves with a custom HTTP wrapper and closes resources on exit.
func (app *App) RunServer(ctx context.Context, server *HTTPServer) error {
	if server == nil {
		return app.runWithStop(10*time.Second, func() error {
			return errors.New("golib: nil HTTP server")
		})
	}
	return app.runWithStop(server.shutdownTimeout, func() error {
		return server.Run(ctx)
	})
}

func (app *App) runWithStop(timeout time.Duration, run func() error) (err error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err = errors.Join(err, app.Stop(stopCtx))
	}()
	return run()
}
