// Package flow -----------------------------
// @file      : cmd.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/3/18 13:54
// -------------------------------------------
package flow

import (
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/server"
	"github.com/xiangtao94/golib/pkg/zlog"
)

func Main(engine *gin.Engine, preFunc func(engine *gin.Engine), postFunc func(engine *gin.Engine)) {
	conf := GetConf()
	if conf == nil {
		panic("need init config: flow.SetDefaultConf() do something?")
	}
	// appName设置
	env.SetAppName(conf.GetAppName())
	// zlog日志初始化
	zlog.InitLog(conf.GetZlogConf())
	// 通用prometheus指标采集接口
	middleware.RegistryMetrics(engine)
	// 全局access中间键日志
	engine.Use(middleware.AccessLog(conf.GetAccessLogConf()))
	// 异常Recovery中间键
	engine.Use(middleware.Recovery(conf.GetHandleRecoveryFunc()))
	// 前置处理
	preFunc(engine)
	// 6.启动
	server.Start(engine, conf.GetPort(), postFunc)
}
