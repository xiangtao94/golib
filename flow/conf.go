// Package flow -----------------------------
// @file      : conf.go
// @author    : xiangtao
// @contact   : xiangtao@hidream.ai
// @time      : 2025/3/18 11:14
// -------------------------------------------
package flow

import (
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
)

var (
	// 默认配置
	DefaultConf IConf
)

type IConf interface {
	// 获取app名称
	GetAppName() string
	// app启动端口
	GetPort() int
	// 通用日志配置
	GetZlogConf() zlog.LogConfig
	// accessLog配置
	GetAccessLogConf() middleware.AccessLoggerConfig
	// 异常捕获方法
	GetHandleRecoveryFunc() gin.RecoveryFunc
}

func GetConf() IConf {
	return DefaultConf
}

func SetDefaultConf(conf IConf) {
	DefaultConf = conf
}
