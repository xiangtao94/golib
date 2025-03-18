package server

import (
	"fmt"
	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"log"
	"strings"
	"syscall"
)

func Start(engine *gin.Engine, port int, postFunc func(engine *gin.Engine)) {
	defer postFunc(engine)
	addr := fmt.Sprintf(":%v", port)
	if strings.Trim(addr, " ") == "" {
		addr = ":8080"
	}
	appServer := endless.NewServer(addr, engine)
	// 监听http端口
	appServer.BeforeBegin = func(add string) {
		log.Println(syscall.Getpid(), "server run", addr)
	}
	// 服务启动
	if err := appServer.ListenAndServe(); err != nil {
		if strings.HasSuffix(err.Error(), "use of closed network connection") {
			return
		}
		panic(err.Error())
	}
}
