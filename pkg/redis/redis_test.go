package redis

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"
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
