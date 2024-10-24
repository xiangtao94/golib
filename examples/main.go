package main

import (
	"github.com/gin-gonic/gin"
	"github.com/tiant-go/golib/examples/conf"
	"github.com/tiant-go/golib/examples/helpers"
	"github.com/tiant-go/golib/examples/router"
	"github.com/tiant-go/golib/flow"
)

func main() {
	defer helpers.Clear()
	// 4.全局变量初始化
	helpers.Init()
	// 1 启动器创建
	engine := gin.New()
	// 6.初始化http服务路由
	router.Http(engine)
	// 5.框架启动
	flow.Start(engine, &conf.WebConf, func(engine *gin.Engine) (err error) {
		flow.SetDefaultDBClient(helpers.MysqlClient)
		flow.SetDefaultRedisClient(helpers.RedisClient)
		return nil
	})
}
