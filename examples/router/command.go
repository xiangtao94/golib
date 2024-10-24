package router

import (
	"github.com/gin-gonic/gin"
	"github.com/tiant-go/golib/pkg/job/cron"
	"github.com/tiant-go/golib/pkg/job/cycle"
)

/*

如果app内需要使用定时任务类，可以通过以下路由加载任务。
* crontab：每个N时间执行一次，不管上次有没有执行完，N时间后就开始执行下一次任务。
  比如：N=2min，任务执行了3min。那么程序启动后2分钟执行一次，任务执行了2分后并未结束，但是又开始执行下一次了。
* cycle：任务执行完后每隔N时间执行一次。
  比如N=2min，任务执行了3min。程序启动时执行第一次，任务执行完后3+2分后才开始执行第二次任务。
需要注意: 除了间隔时间的计算方式不同，第一次执行时间也不同。
*/

func Tasks(engine *gin.Engine) {
	// 定时任务
	//startCrontab(engine)
	startCycle(engine)

}

func startCrontab(engine *gin.Engine) {
	cronJob := cron.InitCrontab(engine)
	cronJob.Start()
	//// 每1秒钟同步redis到db
	//if err := cronJob.AddFunc("0/1 * * * * ?", controllers.DemoJob); err != nil {
	//	log.Fatalf("failed to init SyncRedisToDb cron job: %+v", err)
	//}
}

func startCycle(engine *gin.Engine) {
	cronJob := cycle.InitCycle(engine)

	//cronJob.AddFunc(500*time.Millisecond, controllers.KafKaMinioDo)
	cronJob.Start()
}
