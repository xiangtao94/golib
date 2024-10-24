package server

import (
	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"strings"
)

type ServerConfig struct {
	Address string `yaml:"address"`
}

func (conf *ServerConfig) check() {
	if strings.Trim(conf.Address, " ") == "" {
		conf.Address = ":8080"
	}
}

func Run(engine *gin.Engine, addr string) (err error) {
	conf := &ServerConfig{Address: addr}
	conf.check()
	appServer := endless.NewServer(addr, engine)
	// 监听http端口
	if err := appServer.ListenAndServe(); err != nil {
		return err
	}
	return nil
}
