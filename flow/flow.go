package flow

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib/pkg/conf"
	"strings"

	"github.com/xiangtao94/golib/pkg/server"
)

type StartBeforeFunc func(engine *gin.Engine) (err error)

func Start(engine *gin.Engine, conf conf.IBootstrapConf, startBefore StartBeforeFunc) {
	err := startBefore(engine)
	if err != nil {
		panic(err.Error())
	}
	// 服务启动
	if err = server.Run(engine, fmt.Sprintf(":%v", conf.GetPort())); err != nil {
		if strings.HasSuffix(err.Error(), "use of closed network connection") {
			return
		}
		panic(err.Error())
	}
}
