## 定时任务封装

使用参考
```
package router

import (
	"github.com/tiant-go/go-tiant/job/cron"
	"github.com/tiant-go/go-tiant/job/cycle"
	"github.com/gin-gonic/gin"
	"time"
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
	//if err := cronJob.AddFunc("0 0/5 * * * ?", controllers.LoopThirdData); err != nil {
	//	log.Fatalf("failed to init LoopThirdData cron job: %+v", err)
	//}
}

func startCycle(engine *gin.Engine) {
	cronJob := cycle.InitCycle(engine)
	//cronJob.AddFunc(10*time.Minute, controllers.LoopThirdData)
	cronJob.Start()
}

```