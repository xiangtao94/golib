package flow

import (
	"fmt"
	"github.com/tiant-go/golib/pkg/env"
	"github.com/tiant-go/golib/pkg/redis"
)

const (
	EXPIRE_TIME_1_SECOND  = 1
	EXPIRE_TIME_5_SECOND  = 5
	EXPIRE_TIME_30_SECOND = 30
	EXPIRE_TIME_1_MINUTE  = 60
	EXPIRE_TIME_5_MINUTE  = 300
	EXPIRE_TIME_15_MINUTE = 900
	EXPIRE_TIME_30_MINUTE = 1800
	EXPIRE_TIME_1_HOUR    = 3600
	EXPIRE_TIME_2_HOUR    = 7200
	EXPIRE_TIME_6_HOUR    = 21600
	EXPIRE_TIME_12_HOUR   = 43200
	EXPIRE_TIME_1_DAY     = 86400
	EXPIRE_TIME_3_DAY     = 259200
	EXPIRE_TIME_1_WEEK    = 604800
)

// 默认db
var DefaultRedisClient *redis.Redis

type IRedis interface {
	ILayer
	SetRedis(db *redis.Redis)
	GetRedis() *redis.Redis
}

type Redis struct {
	Layer
	redisCli *redis.Redis
}

func (entity *Redis) SetRedis(cli *redis.Redis) {
	entity.redisCli = cli
}

func (entity *Redis) GetRedis() (cli *redis.Redis) {
	if entity.redisCli != nil {
		cli = entity.redisCli
	} else if DefaultRedisClient != nil {
		cli = DefaultRedisClient
	}
	return cli.WithContext(entity.GetCtx())
}

func (entity *Redis) FormatCacheKey(format string, args ...interface{}) string {
	prefix := getPrefix()
	return fmt.Sprintf(prefix+format, args...)
}

func SetDefaultRedisClient(cli *redis.Redis) {
	DefaultRedisClient = cli
}

func getPrefix() string {
	prefix := ""
	// 模块名默认module，很容易冲突
	if env.GetAppName() == "" {
		prefix += "default:"
	} else {
		prefix += env.GetAppName() + ":"
	}
	return prefix
}
