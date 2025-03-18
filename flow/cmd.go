// Package flow -----------------------------
// @file      : cmd.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/3/18 13:54
// -------------------------------------------
package flow

import (
	"fmt"
	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
	"log"
	"strings"
	"syscall"
)

func Main(engine *gin.Engine, preFunc func(engine *gin.Engine), postFunc func(engine *gin.Engine)) {
	defer postFunc(engine)

	conf := GetConf()
	if conf == nil {
		panic("need init config: flow.SetDefaultConf() do something?")
	}
	addr := fmt.Sprintf(":%v", conf.GetPort())
	if strings.Trim(addr, " ") == "" {
		addr = ":8080"
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
	// 启动前置处理
	preFunc(engine)
	log.Println(syscall.Getpid(), "server run", addr)
	// 6.启动
	appServer := endless.NewServer(addr, engine)
	// 服务启动
	if err := appServer.ListenAndServe(); err != nil {
		if strings.HasSuffix(err.Error(), "use of closed network connection") {
			return
		}
		panic(err.Error())
	}
}
