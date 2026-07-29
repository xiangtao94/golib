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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
)

var ErrNilContext = errors.New("golib: nil context")

type BootstrapConfig struct {
	AppName  string
	Language string
	Log      *zlog.LogConfig

	EnableAccessLog bool
	AccessLog       middleware.AccessLoggerConfig

	EnableRecovery bool
	Recovery       gin.RecoveryFunc

	EnablePrometheus bool
	Collectors       []prometheus.Collector
}

func Bootstrap(engine *gin.Engine, config BootstrapConfig) error {
	if engine == nil {
		return errors.New("golib: nil engine")
	}
	if config.AppName != "" {
		env.SetAppName(config.AppName)
	}
	if config.Language != "" {
		env.SetLanguage(config.Language)
	}
	if config.Log != nil {
		logConfig := *config.Log
		if logConfig.AppName == "" {
			logConfig.AppName = env.GetAppName()
		}
		if _, err := zlog.InitLog(logConfig); err != nil {
			return err
		}
	}

	engine.Use(middleware.RequestID())
	if config.EnableAccessLog {
		engine.Use(middleware.AccessLog(config.AccessLog))
	}
	if config.EnableRecovery {
		engine.Use(middleware.Recovery(
			zlog.NewLoggerWithSkip(1),
			config.Recovery,
		))
	}
	if config.EnablePrometheus {
		metrics, err := middleware.NewMetrics(middleware.MetricsConfig{
			AppName:    env.GetAppName(),
			Collectors: config.Collectors,
		})
		if err != nil {
			return err
		}
		middleware.RegisterMetrics(engine, metrics)
	}
	return nil
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
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
}

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
	if conf.WriteTimeout <= 0 {
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
