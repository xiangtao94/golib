建议按照以下目录规范来规范项目：

```
|- demo
    |-api 访问第三方请求http目录
    |-components 组件，可被其他所有目录依赖
    |-conf 配置文件目录
        |-mount 用来放置环境相关的配置
    |-controllers 控制器目录
        |-http http控制器目录
        |-command 任务控制器入口，包括cycle任务、crontab任务、一次性任务
        |-mq 消息队列回调入口
    |-data 数据层。当项目比较复杂时，可以增加data层用于组装数据，包括不限于数据库查询到的数据、api调用后查询到的数据
    |-helpers 公共类目录，可以用来初始化一些全局变量
    |-models 数据模型访问目录。数据库相关调用。
    |-middleware 业务中间件
    |-router 路由目录，一般对应controllers目录结构
        |-http http路由
        |-command 任务控制器入口
        |-mq 消息队列路由
    |-service 业务逻辑聚合目录。主要强调业务逻辑，能够看出一个功能的核心处理流程。
    |-go.mod go module使用，记录项目的依赖
    |-go.sum go mod tidy 后生成，记录依赖的详细依赖
    |-main.go 程序执行入口
  ```

使用该框架需要注入mysql和redis如下：

```go
    // 框架db默认初始化
layer.SetDefaultDBClient(Db)
// 框架redis默认初始化
layer.SetDefaultRedisClient(Redis)
```
