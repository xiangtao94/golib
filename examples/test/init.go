package test

import (
	"github.com/tiant-go/golib"
	"github.com/tiant-go/golib/examples/conf"
	"github.com/tiant-go/golib/examples/helpers"
	"github.com/tiant-go/golib/flow"
	"github.com/tiant-go/golib/pkg/env"
	"github.com/tiant-go/golib/pkg/zlog"

	"net/http/httptest"
	"path"
	"runtime"
	"sync"

	"github.com/gin-gonic/gin"
)

var once = sync.Once{}
var Ctx *gin.Context

// Init 基础资源初始化
func Init() {
	once.Do(func() {
		engine := gin.New()
		dir := getSourcePath(0)
		env.SetAppName("testing")
		env.SetRootPath(dir + "/..")
		conf.InitConf()
		// 初始化zlog日志
		zlog.InitLog(conf.WebConf.AppName, conf.WebConf.Log)
		helpers.InitResource()
		Ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
		golib.Bootstraps(engine, conf.WebConf)
		flow.SetDefaultDBClient(helpers.MysqlClient)
		flow.SetDefaultRedisClient(helpers.RedisClient)
	})
}

func getSourcePath(skip int) string {
	_, filename, _, _ := runtime.Caller(skip)
	return path.Dir(filename)
}
