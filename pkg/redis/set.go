package redis

import (
	"github.com/gomodule/redigo/redis"
)

// 将一个或多个 member 元素加入到集合 key 当中，已经存在于集合的 member 元素将被忽略
// return: 被添加到集合中的新元素的数量，不包括被忽略的元素
func (r *Redis) SAdd(key string, members ...string) (int64, error) {
	args := packArgs(key, members)
	return redis.Int64(r.Do("SADD", args...))
}

// 判断 member 元素是否集合 key 的成员
func (r *Redis) SIsMember(key string, member string) (bool, error) {
	if res, err := redis.Bool(r.Do("SISMEMBER", key, member)); err == redis.ErrNil {
		return false, nil
	} else {
		return res, err
	}
}

func (r *Redis) SMembers(key string) ([][]byte, error) {
	if res, err := redis.ByteSlices(r.Do("SMembers", key)); err == redis.ErrNil {
		return nil, nil
	} else {
		return res, err
	}
}

// 移除集合 key 中的一个或多个 member 元素，不存在的 member 元素会被忽略
// return: 被成功移除的元素的数量，不包括被忽略的元素
func (r *Redis) SRem(key string, members ...string) (int64, error) {
	args := packArgs(key, members)
	if res, err := redis.Int64(r.Do("SREM", args...)); err == redis.ErrNil {
		return 0, nil
	} else {
		return res, err
	}
}

// 返回集合 key 的基数(集合中元素的数量)
func (r *Redis) SCard(key string) (int64, error) {
	if res, err := redis.Int64(r.Do("SCARD", key)); err == redis.ErrNil {
		return 0, nil
	} else {
		return res, err
	}
}

// 将 member 元素从 source 集合移动到 destination 集合。SMOVE 是原子性操作
func (r *Redis) SMove(source, destination, member string) (bool, error) {
	if res, err := redis.Bool(r.Do("SMOVE", source, destination, member)); err == redis.ErrNil {
		return false, nil
	} else {
		return res, err
	}
}

// 移除并返回集合中的一个随机元素
func (r *Redis) SPop(key string) ([]byte, error) {
	if res, err := redis.Bytes(r.Do("SPOP", key)); err == redis.ErrNil {
		return nil, nil
	} else {
		return res, err
	}
}

// 如果命令执行时，只提供了 key 参数，那么返回集合中的一个随机元素
func (r *Redis) SRandMember(key string) ([]byte, error) {
	if res, err := redis.Bytes(r.Do("SRANDMEMBER", key)); err == redis.ErrNil {
		return nil, nil
	} else {
		return res, err
	}
}

func (r *Redis) SRandMemberCount(key string, count int) ([][]byte, error) {
	if res, err := redis.ByteSlices(r.Do("SRANDMEMBER", key, count)); err == redis.ErrNil {
		return nil, nil
	} else {
		return res, err
	}
}

// 返回一个集合的全部成员，该集合是所有给定集合的交集
func (r *Redis) SInter(keys ...string) ([][]byte, error) {
	args := packArgs(keys)
	if res, err := redis.ByteSlices(r.Do("SINTER", args...)); err == redis.ErrNil {
		return nil, nil
	} else {
		return res, err
	}
}

// 这个命令类似于 SINTER key [key …] 命令，但它将结果保存到 destination 集合，而不是简单地返回结果集
func (r *Redis) SInterStore(dstKey string, keys ...string) (int64, error) {
	args := packArgs(dstKey, keys)
	if res, err := redis.Int64(r.Do("SINTERSTORE", args...)); err == redis.ErrNil {
		return 0, nil
	} else {
		return res, err
	}
}

// 返回一个集合的全部成员，该集合是所有给定集合的并集
func (r *Redis) SUnion(keys ...string) ([][]byte, error) {
	args := packArgs(keys)
	if res, err := redis.ByteSlices(r.Do("SUNION", args...)); err == redis.ErrNil {
		return nil, nil
	} else {
		return res, err
	}
}

// 这个命令类似于 SUNION key [key …] 命令，但它将结果保存到 destination 集合，而不是简单地返回结果集
func (r *Redis) SUnionStore(dstKey string, keys ...string) (int64, error) {
	args := packArgs(dstKey, keys)
	if res, err := redis.Int64(r.Do("SUNIONSTORE", args...)); err == redis.ErrNil {
		return 0, nil
	} else {
		return res, err
	}
}

// 返回一个集合的全部成员，该集合是所有给定集合之间的差集
func (r *Redis) SDiff(keys ...string) ([][]byte, error) {
	args := packArgs(keys)
	if res, err := redis.ByteSlices(r.Do("SDIFF", args...)); err == redis.ErrNil {
		return nil, nil
	} else {
		return res, err
	}
}

// 这个命令的作用和 SDIFF key [key …] 类似，但它将结果保存到 destination 集合，而不是简单地返回结果集
func (r *Redis) SDiffStore(dstKey string, keys ...string) (int64, error) {
	args := packArgs(dstKey, keys)
	if res, err := redis.Int64(r.Do("SDIFFSTORE", args...)); err == redis.ErrNil {
		return 0, nil
	} else {
		return res, err
	}
}

// 基于游标的迭代器，每次被调用会返回新的游标，在下次迭代时，需要使用这个新游标作为游标参数，以此来延续之前的迭代过程
// param: key
// param: cursor 游标 传""表示开始新迭代
// param: count 每次迭代返回元素的最大值，limit hint，实际数量并不准确=count
// param: pattern 模式参数，符合glob风格  ? (一个字符) * （任意个字符） [] (匹配其中的任意一个字符)  \x (转义字符)
// return: 新的cursor，value[]  当返回""，空切片时，表示迭代已结束
func (r *Redis) SScan(key string, cursor uint64, pattern string, count int) (uint64, []string, error) {
	args := packArgs(key, cursor)
	if pattern != "" {
		args = append(args, "MATCH", pattern)
	}
	if count > 0 {
		args = append(args, "COUNT", count)
	}
	values, err := redis.Values(r.Do("SSCAN", args...))
	if err != nil {
		return 0, nil, err
	}
	var items []string
	_, err = redis.Scan(values, &cursor, &items)
	if err != nil {
		return 0, nil, err
	}
	return cursor, items, nil
}
