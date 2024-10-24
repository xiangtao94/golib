package helpers

import (
	"github.com/tiant-go/golib/examples/conf"
	"github.com/tiant-go/golib/pkg/zlog"
)

func Init() {
	// 配置初始化
	conf.InitConf()
	// 初始化mysql
	InitMysql()
	// 初始化redis
	InitRedis()
}

func Clear() {
	// 服务结束时的清理工作，对应 Init() 初始化的资源
	zlog.CloseLogger()
}
