package redis

import (
	"context"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/redis/go-redis/v9"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/zlog"
	"net"
	"time"

	"github.com/gin-gonic/gin"
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

type RedisConf struct {
	Addr            string        `yaml:"addr"`
	Db              int           `yaml:"db"`
	Password        string        `yaml:"password"`
	MaxIdle         int           `yaml:"maxIdle"`
	MaxActive       int           `yaml:"maxActive"`
	IdleTimeout     time.Duration `yaml:"idleTimeout"`
	MaxConnLifetime time.Duration `yaml:"maxConnLifetime"`
	ConnTimeOut     time.Duration `yaml:"connTimeOut"`
	ReadTimeOut     time.Duration `yaml:"readTimeOut"`
	WriteTimeOut    time.Duration `yaml:"writeTimeOut"`
}

func (conf *RedisConf) checkConf() {

	if conf.MaxIdle == 0 {
		conf.MaxIdle = 50
	}
	if conf.MaxActive == 0 {
		conf.MaxActive = 100
	}
	if conf.IdleTimeout == 0 {
		conf.IdleTimeout = 30 * time.Minute
	}
	if conf.MaxConnLifetime == 0 {
		conf.MaxConnLifetime = 10 * time.Minute
	}
	if conf.ConnTimeOut == 0 {
		conf.ConnTimeOut = 3 * time.Second
	}
	if conf.ReadTimeOut == 0 {
		conf.ReadTimeOut = 1200 * time.Millisecond
	}
	if conf.WriteTimeOut == 0 {
		conf.WriteTimeOut = 1200 * time.Millisecond
	}
}

type Redis struct {
	redis.UniversalClient
}

func InitRedisClient(conf RedisConf) (*Redis, error) {
	conf.checkConf()
	redisC := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:           []string{conf.Addr},
		DB:              conf.Db,
		Password:        conf.Password,
		MinIdleConns:    conf.MaxIdle,
		MaxActiveConns:  conf.MaxActive,
		ConnMaxIdleTime: conf.IdleTimeout,
		ConnMaxLifetime: conf.MaxConnLifetime,
		ReadTimeout:     conf.ReadTimeOut,
		DialTimeout:     conf.ConnTimeOut,
		WriteTimeout:    conf.WriteTimeOut,
	})

	redisC.AddHook(newLogger())
	c := &Redis{
		UniversalClient: redisC,
	}
	return c, nil
}

type redisLogger struct {
	logger *zlog.Logger
}

func (r *redisLogger) DialHook(hook redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := hook(ctx, network, addr)
		if err != nil {
			r.logger.Error("get connection error: "+err.Error(), r.commonFields(ctx)...)
		}
		return conn, err
	}
}

func (r *redisLogger) ProcessHook(hook redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		fields := append(r.commonFields(ctx),
			zlog.String("command", cmd.String()),
		)
		msg := "redis"
		start := time.Now()
		err := hook(ctx, cmd)
		if err != nil {
			msg = err.Error()
		}
		fields = append(fields, zlog.AppendCostTime(start, time.Now())...)
		r.logger.Debug(msg, fields...)
		return err
	}
}

func (r *redisLogger) ProcessPipelineHook(hook redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		cmdStrs := []string{}
		for _, c := range cmds {
			cmdStrs = append(cmdStrs, c.String())
		}
		fields := append(r.commonFields(ctx),
			zlog.String("command", slice.Join(cmdStrs, ",")),
		)
		msg := "redis do success"
		start := time.Now()
		err := hook(ctx, cmds)
		if err != nil {
			msg = err.Error()
		}
		fields = append(fields, zlog.AppendCostTime(start, time.Now())...)
		r.logger.Debug(msg, fields...)
		return err
	}
}

func (r *redisLogger) commonFields(ctx context.Context) []zlog.Field {
	var requestID string
	if c, ok := ctx.(*gin.Context); ok && c != nil {
		requestID, _ = ctx.Value(zlog.ContextKeyRequestID).(string)
	}
	return []zlog.Field{
		zlog.String("requestId", requestID),
	}
}

func (r *Redis) Clear() error {
	return r.Close()
}

func newLogger() *redisLogger {
	return &redisLogger{
		logger: zlog.NewLoggerWithSkip(2),
	}
}

func GetKeyPrefix() string {
	prefix := ""
	// 模块名默认module，很容易冲突
	if env.GetAppName() == "" {
		prefix += "default:"
	} else {
		prefix += env.GetAppName() + ":"
	}
	return prefix
}
