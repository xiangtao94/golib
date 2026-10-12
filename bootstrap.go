// Package golib ----------------------------
// @file      : cmd.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/3/18 13:54
// -------------------------------------------
package golib

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/health"
	"github.com/xiangtao94/golib/pkg/lifecycle"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
)

var ErrNilContext = errors.New("golib: nil context")

type BootstrapOption func(*bootstrapOptions) error

type bootstrapOptions struct {
	language          string
	logConfig         *zlog.LogConfig
	accessLogConfig   *middleware.AccessLoggerConfig
	recoveryHandler   gin.RecoveryFunc
	metricsCollectors []prometheus.Collector
	metricsRequested  bool
	readinessGate     *health.Gate
}

// WithLang sets the process default language. Request-specific language may
// still be set through env.WithLanguage.
func WithLang(lang string) BootstrapOption {
	return func(options *bootstrapOptions) error {
		options.language = lang
		return nil
	}
}

// WithZlog customizes the process logger initialized by Bootstraps.
func WithZlog(conf zlog.LogConfig) BootstrapOption {
	return func(options *bootstrapOptions) error {
		options.logConfig = &conf
		return nil
	}
}

// WithAccessLog installs access logging before panic recovery.
func WithAccessLog(conf ...middleware.AccessLoggerConfig) BootstrapOption {
	return func(options *bootstrapOptions) error {
		logConfig := middleware.DefaultAccessLoggerConfig()
		if len(conf) > 0 {
			logConfig = conf[0]
		}
		options.accessLogConfig = &logConfig
		return nil
	}
}

// WithRecovery customizes the recovery installed by Bootstraps.
func WithRecovery(handler gin.RecoveryFunc) BootstrapOption {
	return func(options *bootstrapOptions) error {
		options.recoveryHandler = handler
		return nil
	}
}

// WithPrometheus installs HTTP metrics and registers /metrics.
func WithPrometheus(cs ...prometheus.Collector) BootstrapOption {
	return func(options *bootstrapOptions) error {
		if options.metricsRequested {
			return errors.New("golib: prometheus already configured")
		}
		options.metricsCollectors = cs
		options.metricsRequested = true
		return nil
	}
}

// WithReadiness registers /readyz with an application-owned readiness gate.
// The gate starts not ready; the application marks it ready after startup and
// not ready before draining.
func WithReadiness(gate *health.Gate) BootstrapOption {
	return func(options *bootstrapOptions) error {
		if gate == nil {
			return errors.New("golib: nil readiness gate")
		}
		options.readinessGate = gate
		return nil
	}
}

// Bootstraps configures a Gin engine before business routes are registered.
// appName is required and supplies the default name for logging and metrics.
// The returned App owns logger shutdown; call RunServer or Stop to release it.
func Bootstraps(engine *gin.Engine, appName string, opts ...BootstrapOption) (*App, error) {
	if engine == nil {
		return nil, errors.New("golib: nil engine")
	}
	if strings.TrimSpace(appName) == "" {
		return nil, errors.New("golib: app name is required")
	}
	if appName != strings.TrimSpace(appName) || appName == "." || appName == ".." ||
		strings.ContainsAny(appName, "/\\") {
		return nil, errors.New("golib: invalid app name")
	}
	options := &bootstrapOptions{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	var metrics *middleware.Metrics
	if options.metricsRequested {
		var err error
		metrics, err = middleware.NewMetrics(middleware.MetricsConfig{
			AppName:    appName,
			Collectors: options.metricsCollectors,
		})
		if err != nil {
			return nil, err
		}
	}
	logConfig := zlog.DefaultLogConfig()
	if options.logConfig != nil {
		logConfig = *options.logConfig
	}
	logConfig.AppName = appName
	if _, err := zlog.InitLog(logConfig); err != nil {
		return nil, err
	}
	env.SetAppName(appName)
	if options.language != "" {
		env.SetLanguage(options.language)
	}
	manager, err := lifecycle.New(lifecycle.Hook{
		Name:   "zlog",
		OnStop: func(context.Context) error { return zlog.CloseLogger() },
	})
	if err != nil {
		_ = zlog.CloseLogger()
		return nil, err
	}
	if err := manager.Start(context.Background()); err != nil {
		_ = zlog.CloseLogger()
		return nil, err
	}

	engine.Use(middleware.RequestID())
	if options.accessLogConfig != nil {
		engine.Use(middleware.AccessLog(*options.accessLogConfig))
	}
	if metrics != nil {
		engine.Use(metrics.Middleware())
	}
	engine.Use(middleware.Recovery(zlog.NewLoggerWithSkip(1), options.recoveryHandler))
	engine.GET("/livez", gin.WrapH(health.New().LivenessHandler()))
	if options.readinessGate != nil {
		engine.GET("/readyz", gin.WrapH(options.readinessGate.ReadinessHandler()))
	}
	if metrics != nil {
		middleware.RegisterMetricsRoute(engine, metrics)
	}
	return &App{Engine: engine, manager: manager}, nil
}

// RegisterPprof explicitly registers pprof handlers on the supplied engine.
// Mount this only on an authenticated, network-isolated admin listener.
func RegisterPprof(engine *gin.Engine) {
	engine.GET("/debug/pprof/", gin.WrapF(pprof.Index))
	engine.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	engine.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	engine.POST("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	engine.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	engine.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))

	for _, profile := range []string{
		"allocs",
		"block",
		"goroutine",
		"heap",
		"mutex",
		"threadcreate",
	} {
		engine.GET("/debug/pprof/"+profile, gin.WrapH(pprof.Handler(profile)))
	}
}

type HTTPServerConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	// WriteTimeout is the maximum time allowed to write a response. Set it to
	// NoWriteTimeout for long-lived streams such as SSE or MCP. Zero uses the
	// default timeout.
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxHeaderBytes  int
}

// NoWriteTimeout disables the HTTP server's response write deadline. Use it
// only when handlers manage the lifetime of long-lived connections.
const NoWriteTimeout time.Duration = -1

func DefaultHTTPServerConfig(port int) HTTPServerConfig {
	if port <= 0 {
		port = 8080
	}
	return HTTPServerConfig{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

type HTTPServer struct {
	server          *http.Server
	shutdownTimeout time.Duration
}

func NewHTTPServer(handler http.Handler, conf HTTPServerConfig) *HTTPServer {
	defaults := DefaultHTTPServerConfig(8080)
	if conf.Addr == "" {
		conf.Addr = defaults.Addr
	}
	if conf.ReadHeaderTimeout <= 0 {
		conf.ReadHeaderTimeout = defaults.ReadHeaderTimeout
	}
	if conf.ReadTimeout <= 0 {
		conf.ReadTimeout = defaults.ReadTimeout
	}
	if conf.WriteTimeout == NoWriteTimeout {
		conf.WriteTimeout = 0
	} else if conf.WriteTimeout <= 0 {
		conf.WriteTimeout = defaults.WriteTimeout
	}
	if conf.IdleTimeout <= 0 {
		conf.IdleTimeout = defaults.IdleTimeout
	}
	if conf.ShutdownTimeout <= 0 {
		conf.ShutdownTimeout = defaults.ShutdownTimeout
	}
	if conf.MaxHeaderBytes <= 0 {
		conf.MaxHeaderBytes = defaults.MaxHeaderBytes
	}

	return &HTTPServer{
		server: &http.Server{
			Addr:              conf.Addr,
			Handler:           handler,
			ReadHeaderTimeout: conf.ReadHeaderTimeout,
			ReadTimeout:       conf.ReadTimeout,
			WriteTimeout:      conf.WriteTimeout,
			IdleTimeout:       conf.IdleTimeout,
			MaxHeaderBytes:    conf.MaxHeaderBytes,
		},
		shutdownTimeout: conf.ShutdownTimeout,
	}
}

func (s *HTTPServer) Run(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.server.Addr, err)
	}
	return s.Serve(ctx, listener)
}

func (s *HTTPServer) Serve(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		return ErrNilContext
	}

	serveResult := make(chan error, 1)
	go func() {
		serveResult <- s.server.Serve(listener)
	}()

	select {
	case err := <-serveResult:
		return normalizeServerError(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()

		shutdownErr := s.server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = s.server.Close()
		}
		serveErr := normalizeServerError(<-serveResult)
		return errors.Join(shutdownErr, serveErr)
	}
}

func normalizeServerError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
