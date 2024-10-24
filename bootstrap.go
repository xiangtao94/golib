package golib

import (
	"github.com/gin-gonic/gin"
	"github.com/tiant-go/golib/pkg/conf"
	"github.com/tiant-go/golib/pkg/env"
	"github.com/tiant-go/golib/pkg/middleware"
	"github.com/tiant-go/golib/pkg/zlog"
)

// 全局注册一下
func Bootstraps(engine *gin.Engine, conf conf.IBootstrapConf) {
	// appName设置
	env.SetAppName(conf.GetAppName())
	// zlog日志初始化
	zlog.InitLog(conf.GetAppName(), conf.GetZlogConf())
	// 通用prometheus指标采集接口
	middleware.RegistryMetrics(engine, conf.GetAppName())
	// 全局access中间键日志
	engine.Use(middleware.AccessLog(conf.GetAccessLogConf()))
	// 异常Recovery中间键
	engine.Use(middleware.Recovery(conf.GetHandleRecoveryFunc()))
}
