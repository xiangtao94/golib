package flow

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"strings"

	"github.com/xiangtao94/golib/pkg/server"
)

func Start(engine *gin.Engine, port int) {
	var err error
	// 服务启动
	if err = server.Run(engine, fmt.Sprintf(":%v", port)); err != nil {
		if strings.HasSuffix(err.Error(), "use of closed network connection") {
			return
		}
		panic(err.Error())
	}
}
