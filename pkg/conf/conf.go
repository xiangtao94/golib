package conf

import (
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/middleware"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type IBootstrapConf interface {
	// 获取app名称
	GetAppName() string
	// app启动端口
	GetPort() int
	// 通用配置
	GetZlogConf() zlog.LogConfig
	// accessLog配置
	GetAccessLogConf() middleware.AccessLoggerConfig
	// 异常捕获方法
	GetHandleRecoveryFunc() gin.RecoveryFunc
}
