package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRedisRequiresTLSByDefault(t *testing.T) {
	_, err := buildRedisOptions(RedisConf{Addr: "localhost:6379"})
	require.Error(t, err)
}

func TestRedisClonesTLSConfigAndSetsMinimumVersion(t *testing.T) {
	source := &tls.Config{}
	options, err := buildRedisOptions(RedisConf{Addr: "redis.example.com:6379", TLSConfig: source})

	require.NoError(t, err)
	require.NotSame(t, source, options.TLSConfig)
	require.Equal(t, uint16(tls.VersionTLS12), options.TLSConfig.MinVersion)
	require.Zero(t, source.MinVersion)
}

func TestRedisLoggerDoesNotFormatCommandsWhenDebugIsDisabled(t *testing.T) {
	command := &stringTrackingCommand{
		Cmder: goredis.NewStringCmd(context.Background(), "set", "key", "secret"),
	}
	logger := &redisLogger{logger: zap.NewNop()}
	hook := logger.ProcessHook(func(context.Context, goredis.Cmder) error {
		return nil
	})

	require.NoError(t, hook(context.Background(), command))
	require.Zero(t, command.stringCalls)
}

func TestRedisLoggerRecordsMetadataWithoutCommandArguments(t *testing.T) {
	core, observed := observer.New(zap.DebugLevel)
	logger := &redisLogger{logger: zap.New(core)}
	command := goredis.NewStringCmd(context.Background(), "set", "key", "secret")
	hook := logger.ProcessHook(func(context.Context, goredis.Cmder) error {
		return nil
	})

	require.NoError(t, hook(context.Background(), command))

	fields := observed.All()[0].ContextMap()
	require.Equal(t, "set", fields["command"])
	require.EqualValues(t, 3, fields["argumentCount"])
	for _, value := range fields {
		require.NotContains(t, fmt.Sprint(value), "secret")
	}
}

type stringTrackingCommand struct {
	goredis.Cmder
	stringCalls int
}

func (command *stringTrackingCommand) String() string {
	command.stringCalls++
	return command.Cmder.String()
}
