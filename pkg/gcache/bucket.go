package gcache

import (
	"runtime"
	"time"
)

type BucketCache struct {
	*shardedCache
}

func NewBucketCache(defaultExpiration, cleanupInterval time.Duration, shardnum int) *BucketCache {
	if defaultExpiration == 0 {
		defaultExpiration = -1
	}
	sc := newShardedCache(shardnum, defaultExpiration)
	SC := &BucketCache{sc}
	if cleanupInterval > 0 {
		runShardedJanitor(sc, cleanupInterval)
		runtime.SetFinalizer(SC, stopShardedJanitor)
	}
	return SC
}

// Close 停止janitor goroutine并释放资源
func (bc *BucketCache) Close() {
	if bc.janitor != nil {
		stopShardedJanitor(bc)
		bc.janitor = nil
	}
	runtime.SetFinalizer(bc, nil)
}
