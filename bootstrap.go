// Package flow -----------------------------
// @file      : cmd.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/3/18 13:54
// -------------------------------------------
package golib

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
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

// 3. 日志
func WithZlog(conf zlog.LogConfig) BootstrapOption {
	return func(engine *gin.Engine) {
		zlog.InitLog(conf)
	}
}

// 4. Access Log
func WithAccessLog(conf middleware.AccessLoggerConfig) BootstrapOption {
	return func(engine *gin.Engine) {
		middleware.RegistryAccessLog(engine, conf)
	}
}

// 5. Recovery
func WithRecovery(handler gin.RecoveryFunc) BootstrapOption {
	return func(engine *gin.Engine) {
		middleware.RegistryRecovery(engine, handler)
	}
}

// 6. Swagger
func WithSwagger(urlPrefix string) BootstrapOption {
	return func(engine *gin.Engine) {
		if urlPrefix == "" {
			urlPrefix = "/swagger"
		}
		engine.GET(urlPrefix+"/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
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
	server := endless.NewServer(addr, engine)
	server.BeforeBegin = func(add string) {
		log.Printf("PID: %d, Server running at %s\n", os.Getpid(), addr)
	}
	if err := server.ListenAndServe(); err != nil {
		if strings.HasSuffix(err.Error(), "use of closed network connection") {
			log.Printf("Server at %s closed normally\n", addr)
			return nil
		}
		log.Printf("Server at %s failed: %v\n", addr, err)
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}
