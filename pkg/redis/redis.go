package redis

import (
	"context"
	"fmt"
	"github.com/tiant-go/golib/pkg/zlog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	redigo "github.com/gomodule/redigo/redis"
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
		conf.IdleTimeout = 5 * time.Minute
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

// 日志打印Do args部分支持的最大长度
type Redis struct {
	pool   *redigo.Pool
	logger *zlog.Logger
	ctx    context.Context
}

func (r *Redis) WithContext(ctx context.Context) *Redis {
	r.ctx = ctx
	return r
}

func InitRedisClient(conf RedisConf) (*Redis, error) {
	conf.checkConf()
	p := &redigo.Pool{
		MaxIdle:         conf.MaxIdle,
		MaxActive:       conf.MaxActive,
		IdleTimeout:     conf.IdleTimeout,
		MaxConnLifetime: conf.MaxConnLifetime,
		Wait:            true,
		Dial: func() (conn redigo.Conn, e error) {
			con, err := redigo.Dial(
				"tcp",
				conf.Addr,
				redigo.DialPassword(conf.Password),
				redigo.DialConnectTimeout(conf.ConnTimeOut),
				redigo.DialReadTimeout(conf.ReadTimeOut),
				redigo.DialWriteTimeout(conf.WriteTimeOut),
				redigo.DialDatabase(conf.Db),
			)
			if err != nil {
				return nil, err
			}
			return con, nil
		},
		TestOnBorrow: func(c redigo.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
	c := &Redis{
		pool:   p,
		logger: zlog.ZapLogger.WithOptions(zlog.AddCallerSkip(1)),
		ctx:    context.Background(),
	}
	return c, nil
}

func (r *Redis) Do(commandName string, args ...interface{}) (reply interface{}, err error) {
	start := time.Now()
	conn := r.pool.Get()
	if err := conn.Err(); err != nil {
		r.logger.Error("get connection error: "+err.Error(), r.commonFields(r.ctx)...)
		return reply, err
	}
	reply, err = conn.Do(commandName, args...)
	if e := conn.Close(); e != nil {
		r.logger.Warn("connection close error: "+e.Error(), r.commonFields(r.ctx)...)
	}
	// 执行时间 单位:毫秒
	msg := "redis do success"
	if err != nil {
		// 超时不报错
		if commandName != "BLPOP" && commandName != "BRPOP" && !strings.Contains(err.Error(), "i/o timeout") {
			msg = "redis do error: " + err.Error()
			r.logger.Error(msg, r.commonFields(r.ctx)...)
		}
	}
	fields := append(r.commonFields(r.ctx),
		zlog.String("command", commandName),
		zlog.String("commandVal", joinArgs(500, args...)),
	)
	fields = append(fields, zlog.AppendCostTime(start, time.Now())...)
	r.logger.Debug(msg, fields...)
	return reply, err
}

func joinArgs(showByte int, args ...interface{}) string {
	var sumLen int

	argStr := make([]string, len(args))
	for i, v := range args {
		if s, ok := v.(string); ok {
			argStr[i] = s
		} else {
			argStr[i] = fmt.Sprintf("%v", v)
		}

		sumLen += len(argStr[i])
	}

	argVal := strings.Join(argStr, " ")
	if sumLen > showByte {
		argVal = argVal[:showByte] + " ..."
	}
	return argVal
}

func (r *Redis) commonFields(ctx context.Context) []zlog.Field {
	var requestID string
	if c, ok := ctx.(*gin.Context); ok && c != nil {
		requestID, _ = ctx.Value(zlog.ContextKeyRequestID).(string)
	}
	return []zlog.Field{
		zlog.String("requestId", requestID),
	}
}

func (r *Redis) Close() error {
	return r.pool.Close()
}

func (r *Redis) Stats() (inUseCount, idleCount, activeCount int) {
	stats := r.pool.Stats()
	idleCount = stats.IdleCount
	activeCount = stats.ActiveCount
	inUseCount = activeCount - idleCount
	return inUseCount, idleCount, activeCount
}
