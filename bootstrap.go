// Package flow -----------------------------
// @file      : cmd.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/3/18 13:54
// -------------------------------------------
package golib

import (
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type IBootstrapConf interface {
	// 获取app名称
	GetAppName() string
	// 国际化
	GetLang() string
	// app启动端口
	GetPort() int
	// 通用日志配置
	GetZlogConf() zlog.LogConfig
	// accessLog配置
	GetAccessLogConf() middleware.AccessLoggerConfig
	// 异常捕获方法
	GetHandleRecoveryFunc() gin.RecoveryFunc
}

func Bootstraps(engine *gin.Engine, conf IBootstrapConf) {
	// appName设置
	env.SetAppName(conf.GetAppName())
	// 国际化设置
	env.SetLanguage(conf.GetLang())
	// 日志初始化
	zlog.InitLog(conf.GetZlogConf())
	// 通用prometheus指标采集接口
	middleware.RegistryMetrics(engine)
	// 全局access中间键日志
	middleware.RegistryAccessLog(engine, conf.GetAccessLogConf())
	// 异常Recovery中间键
	middleware.RegistryRecovery(engine, conf.GetHandleRecoveryFunc())
	// 添加swagger
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
}
