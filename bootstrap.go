package golib

import (
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/conf"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
)

// 全局注册一下
func Bootstraps(engine *gin.Engine, conf conf.IBootstrapConf) *gin.Engine {
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
	return engine
}
