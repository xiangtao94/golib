package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/xiangtao94/golib/pkg/zlog"
)

type RedisConf struct {
	Addr                   string        `yaml:"addr"`
	Db                     int           `yaml:"db"`
	Password               string        `yaml:"password"`
	MaxIdle                int           `yaml:"maxIdle"`
	MaxActive              int           `yaml:"maxActive"`
	IdleTimeout            time.Duration `yaml:"idleTimeout"`
	MaxConnLifetime        time.Duration `yaml:"maxConnLifetime"`
	ConnTimeOut            time.Duration `yaml:"connTimeOut"`
	ReadTimeOut            time.Duration `yaml:"readTimeOut"`
	WriteTimeOut           time.Duration `yaml:"writeTimeOut"`
	MaxRetries             int           `yaml:"maxRetries"`
	TLSConfig              *tls.Config   `yaml:"-"`
	AllowInsecureTransport bool          `yaml:"allowInsecureTransport"`
}

func (conf *RedisConf) checkConf() {
	if conf.MaxIdle <= 0 {
		conf.MaxIdle = 10
	}
	if conf.MaxActive <= 0 {
		conf.MaxActive = 50
	}
	if conf.IdleTimeout <= 0 {
		conf.IdleTimeout = 5 * time.Minute
	}
	if conf.MaxConnLifetime <= 0 {
		conf.MaxConnLifetime = 30 * time.Minute
	}
	if conf.ConnTimeOut <= 0 {
		conf.ConnTimeOut = 3 * time.Second
	}
	if conf.ReadTimeOut <= 0 {
		conf.ReadTimeOut = 2 * time.Second
	}
	if conf.WriteTimeOut <= 0 {
		conf.WriteTimeOut = 2 * time.Second
	}
	if conf.MaxRetries < 0 {
		conf.MaxRetries = 3
	}
}

type Redis struct {
	redis.UniversalClient
}

func InitRedisClient(conf RedisConf) (*Redis, error) {
	conf.checkConf()

	opts, err := buildRedisOptions(conf)
	if err != nil {
		return nil, err
	}

	rdb := redis.NewUniversalClient(opts)
	rdb.AddHook(newLogger())

	// Ping 测试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping error: %w", err)
	}

	return &Redis{UniversalClient: rdb}, nil
}

func buildRedisOptions(conf RedisConf) (*redis.UniversalOptions, error) {
	if conf.TLSConfig == nil && !conf.AllowInsecureTransport {
		return nil, fmt.Errorf("redis: TLS is required; configure TLSConfig or explicitly allow insecure transport")
	}
	var tlsConfig *tls.Config
	if conf.TLSConfig != nil {
		tlsConfig = conf.TLSConfig.Clone()
		if tlsConfig.MinVersion == 0 {
			tlsConfig.MinVersion = tls.VersionTLS12
		}
	}
	return &redis.UniversalOptions{
		Addrs:           strings.Split(conf.Addr, ","),
		DB:              conf.Db,
		Password:        conf.Password,
		MinIdleConns:    conf.MaxIdle,
		PoolSize:        conf.MaxActive,
		ConnMaxIdleTime: conf.IdleTimeout,
		ConnMaxLifetime: conf.MaxConnLifetime,
		ReadTimeout:     conf.ReadTimeOut,
		DialTimeout:     conf.ConnTimeOut,
		WriteTimeout:    conf.WriteTimeOut,
		MaxRetries:      conf.MaxRetries,
		TLSConfig:       tlsConfig,
	}, nil
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
		start := time.Now()
		err := hook(ctx, cmd)
		if !r.logger.Core().Enabled(zap.DebugLevel) {
			return err
		}
		msg := "redis"
		if err != nil {
			msg = err.Error()
		}
		fields := append(r.commonFields(ctx),
			zlog.String("command", cmd.Name()),
			zlog.Int("argumentCount", len(cmd.Args())),
		)
		fields = append(fields, zlog.String("cost", fmt.Sprintf("%vms", time.Since(start).Seconds()*1000)))
		r.logger.Debug(msg, fields...)
		return err
	}
}

func (r *redisLogger) ProcessPipelineHook(hook redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := hook(ctx, cmds)
		if !r.logger.Core().Enabled(zap.DebugLevel) {
			return err
		}
		commandNames := make([]string, 0, len(cmds))
		argumentCount := 0
		for _, command := range cmds {
			commandNames = append(commandNames, command.Name())
			argumentCount += len(command.Args())
		}
		msg := "redis do success"
		if err != nil {
			msg = err.Error()
		}
		fields := append(r.commonFields(ctx),
			zlog.Strings("commands", commandNames),
			zlog.Int("commandCount", len(cmds)),
			zlog.Int("argumentCount", argumentCount),
		)
		fields = append(fields, zlog.String("cost", fmt.Sprintf("%vms", time.Since(start).Seconds()*1000)))
		r.logger.Debug(msg, fields...)
		return err
	}
}

func (r *redisLogger) commonFields(ctx context.Context) []zlog.Field {
	requestID := ""
	if ctx != nil {
		requestID = zlog.GetRequestID(ctx)
	}
	return []zlog.Field{
		zlog.String("requestId", requestID),
	}
}

func newLogger() *redisLogger {
	return &redisLogger{
		logger: zlog.NewLoggerWithSkip(2),
	}
}
