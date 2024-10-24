package redis

import (
	"github.com/gomodule/redigo/redis"
)

func (r *Redis) Expire(key string, time int64) (bool, error) {
	return redis.Bool(r.Do("EXPIRE", key, time))
}

func (r *Redis) Exists(key string) (bool, error) {
	return redis.Bool(r.Do("EXISTS", key))
}

func (r *Redis) Del(keys ...interface{}) (int64, error) {
	return redis.Int64(r.Do("DEL", keys...))
}

func (r *Redis) Ttl(key string) (int64, error) {
	return redis.Int64(r.Do("TTL", key))
}

func (r *Redis) Pttl(key string) (int64, error) {
	return redis.Int64(r.Do("PTTL", key))
}
