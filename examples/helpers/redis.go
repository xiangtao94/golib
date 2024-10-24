package helpers

import (
	"github.com/tiant-go/golib/examples/conf"
	"github.com/tiant-go/golib/pkg/redis"
)

// 推荐，直接使用
var RedisClient *redis.Redis

// 初始化redis
func InitRedis() {
	var err error
	for name, redisConf := range conf.WebConf.Redis {
		switch name {
		case "default":
			RedisClient, err = redis.InitRedisClient(redisConf)
		}
		if err != nil {
			panic("redis connect error: %v" + err.Error())
		}
	}
}

func CloseRedis() {
	_ = RedisClient.Close()
}
