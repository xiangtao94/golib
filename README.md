# golib

**golib** 是一个基于 Go 和 Gin 框架封装的实用库，旨在提高开发效率，提供常用功能组件，简化业务开发。

## 项目地址

GitHub: [https://github.com/xiangtao94/golib](https://github.com/xiangtao94/golib)

## 功能点

该库提供了一系列常用组件和工具，适用于 Go 语言项目开发：

### 2. **Elasticsearch 支持 (`elastic8`)**
- 封装了对 Elasticsearch 8.x 版本的支持，简化索引、搜索等操作。

### 3. **环境管理 (`env`)**
- 统一管理环境变量，支持开发、测试、生产等多种环境配置。

### 4. **错误处理 (`errors`)**
- 提供统一的错误处理方法和自定义错误类型，便于错误跟踪和日志记录。

### 5. **缓存组件 (`gcache`)**
- 封装了缓存管理，支持本地缓存及扩展。

### 6. **HTTP 请求处理 (`http`)**
- 基于 Gin 框架封装了常用的 HTTP 请求方法，支持 RESTful 接口开发。
- 提供中间件支持，如请求日志、CORS 处理等。
- 

### 7. **任务调度 (`job`)**
- 支持任务调度和管理，可用于定时任务处理。

### 8. **中间件 (`middleware`)**
- 提供常用的中间件封装，例如：
    - AccessLog日志记录
    - 异常恢复
    - 跨域支持 (CORS)

### 9. **ORM 封装 (`orm`)**
- 基于 GORM 提供数据库操作封装，支持多种数据库(mysql)。

### 10. **Redis 支持 (`redis`)**
- 封装 Redis 客户端，简化 Redis 读写操作，支持缓存管理。

### 11. **消息队列 (`rmq`)**
- 支持消息队列(rocketMQ)封装，便于实现异步消息处理。

### 12. **服务端工具 (`server`)**
- 提供 HTTP 服务启动封装，快速启动 Gin 服务。已接入 endless 做平滑启停

### 13. **Server-Sent Events (`sse`)**
- 支持服务端推送事件，便于实现实时通知功能。

### 14. **工具方法集 (`util`)**
- 提供常用的工具函数，包括字符串处理、时间格式化等。

### 15. **日志管理 (`zlog`)**
- 封装日志组件，支持分级、格式化日志输出（JSON），支持上下文requestId, 方便跟踪与调试。

---

## 目录结构

```plaintext
golib/
├── flow/              # 封装的面向对象的业务框架，简化开发逻辑
├── pkg/
│   ├── elastic7/      # Elasticsearch 7.x 支持
│   ├── elastic8/      # Elasticsearch 8.x 支持
│   ├── env/           # 环境管理
│   ├── errors/        # 错误处理
│   ├── gcache/        # 缓存管理
│   ├── http/          # HTTP 封装
│   ├── job/           # 任务调度
│   ├── middleware/    # 中间件
│   ├── orm/           # ORM 封装
│   ├── redis/         # Redis 支持
│   ├── rmq/           # 消息队列
│   ├── server/        # web服务启动
│   ├── sse/           # Even-Stream流式支持
│   ├── util/          # 工具方法集
│   └── zlog/          # 日志管理, 封装统一上下文
```

---

## 安装

使用 `go get` 下载：

```bash
go get -u github.com/xiangtao94/golib
```

### 1. 使用redis

```go
package main

import (
	"github.com/xiangtao94/golib/pkg/redis"
)

var RedisClient *redis.Redis

func main() {
	var err error
	RedisClient, err = redis.InitRedisClient(redis.RedisConf{
		Addr: "127.0.0.1:6379",
	})
	if err != nil {
        panic(err.Error())
	}
}
```

### 2. 使用mysql

```go
package main

import (
	"github.com/xiangtao94/golib/pkg/orm"
	"gorm.io/gorm"
)
var (
	MysqlClient *gorm.DB
)

func main() {
	var err error
	MysqlClient, err = orm.InitMysqlClient(orm.MysqlConf{
		Addr: "127.0.0.1:3306",
    })
	if err != nil {
        panic(err.Error())
	}
}
```

### 2. 使用flow框架构建web服务, 已经包含JSON类型日志框架+请求上下文

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/xiangtao94/golib"
	"github.com/xiangtao94/golib/flow"
	"github.com/xiangtao94/golib/pkg/conf"
	"github.com/xiangtao94/golib/pkg/zlog"
	"github.com/xiangtao94/golib/pkg/middleware"
)

type SWebConf struct {
}

func (s *SWebConf) GetZlogConf() zlog.LogConfig {
	return zlog.LogConfig{}
}

func (s *SWebConf) GetAccessLogConf() middleware.AccessLoggerConfig {
	return middleware.AccessLoggerConfig{}
}

func (s *SWebConf) GetHandleRecoveryFunc() gin.RecoveryFunc {
	return nil
}

func (s *SWebConf) GetAppName() string {
	return "demo"
}

func (s *SWebConf) GetPort() int {
	return 8080
}

func main() {
	engine := gin.New()
	defaultConf :=  SWebConf{}
	golib.Bootstraps(engine,defaultConf)
	flow.Start(engine, defaultConf, nil)
	}
```
## 贡献

欢迎提交 Issue 和 Pull Request，共同完善这个库。

---

## 许可

本项目基于 MIT 许可开源。详细内容请参考 [LICENSE](./LICENSE)。

---

## 未来规划

- 增加更多中间件支持
- 扩展更多数据库及消息队列
- 提供更多工具类函数

---

如果你觉得这个库对你有帮助，请给个 **Star ⭐️** 支持！

---
