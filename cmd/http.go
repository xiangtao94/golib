// Package cmd -----------------------------
// @file      : http.go
// @author    : xiangtao
// @time      : 2025/4/2 13:21
// -------------------------------------------
package cmd

import (
	"fmt"
	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"log"
	"strings"
	"syscall"
)

func StartHttpServer(engine *gin.Engine, port int) {
	addr := fmt.Sprintf(":%v", port)
	if strings.Trim(addr, " ") == "" {
		addr = ":8080"
	}
	appServer := endless.NewServer(addr, engine)
	appServer.BeforeBegin = func(add string) {
		log.Println(syscall.Getpid(), "server run", addr)
	}
	if err := appServer.ListenAndServe(); err != nil {
		if strings.HasSuffix(err.Error(), "use of closed network connection") {
			return
		}
		panic(err.Error())
	}
}
