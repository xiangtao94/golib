// Package flow -----------------------------
// @file      : cmd.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/3/18 13:54
// -------------------------------------------
package golib

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"

	_ "net/http/pprof"
)

type BootstrapOption func(engine *gin.Engine)

// 1. 应用名称
func WithAppName(appName string) BootstrapOption {
	return func(engine *gin.Engine) {
		env.SetAppName(appName)
	}
}

// 2. 国际化环境
func WithLang(lang string) BootstrapOption {
	return func(engine *gin.Engine) {
		env.SetLanguage(lang)
	}
}

// 3. 日志 - 支持可选配置
func WithZlog(conf ...zlog.LogConfig) BootstrapOption {
	return func(engine *gin.Engine) {
		zlog.InitLog(conf...)
	}
}

// 4. Access Log - 支持可选配置
func WithAccessLog(conf ...middleware.AccessLoggerConfig) BootstrapOption {
	return func(engine *gin.Engine) {
		middleware.RegistryAccessLog(engine, conf...)
	}
}

// 5. Recovery
func WithRecovery(handler gin.RecoveryFunc) BootstrapOption {
	return func(engine *gin.Engine) {
		middleware.RegistryRecovery(engine, handler)
	}
}

// 6. Prometheus
func WithPrometheus(cs ...prometheus.Collector) BootstrapOption {
	return func(engine *gin.Engine) {
		// 统一的Prometheus注册
		middleware.RegistryMetrics(engine, cs...)
	}
}

func Bootstraps(engine *gin.Engine, opts ...BootstrapOption) {
	// 依次执行传入的可选项
	for _, opt := range opts {
		opt(engine)
	}
	// 统一添加pprof
	engine.GET("/debug/pprof/*any", gin.WrapH(http.DefaultServeMux))
}

func StartHttpServer(engine *gin.Engine, port int) error {
	addr := fmt.Sprintf(":%d", port)
	if strings.TrimSpace(addr) == "" || addr == ":" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	zlog.Info(nil, "Server is running on %s", addr)
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zlog.Info(nil, "Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zlog.Error(nil, "Server forced to shutdown: %v", err)
	}

	zlog.Info(nil, "Server exiting")
	return nil
}
