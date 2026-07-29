// Package gcache provides a typed, sharded in-memory cache.
package gcache

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	// NoExpiration keeps an item until it is explicitly deleted.
	NoExpiration time.Duration = -1
	// DefaultExpiration uses the duration configured when the cache was created.
	DefaultExpiration time.Duration = 0
)

var (
	ErrNotFound = errors.New("gcache: item not found")
	ErrExists   = errors.New("gcache: item already exists")
)

// Item is a serializable cache entry.
type Item[V any] struct {
	Value      V
	Expiration int64
}

// Expired reports whether the item is expired at the current time.
func (item Item[V]) Expired() bool {
	return item.expiredAt(time.Now().UnixNano())
}

func (item Item[V]) expiredAt(now int64) bool {
	return item.Expiration > 0 && now > item.Expiration
}

type shard[V any] struct {
	mu    sync.RWMutex
	items map[string]Item[V]
}

type cacheState[V any] struct {
	defaultExpiration time.Duration
	seed              uint32
	shards            []shard[V]

	evictionMu sync.RWMutex
	onEvicted  func(string, V)
}

type janitor[V any] struct {
	state    *cacheState[V]
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// Cache is a typed cache whose keys are distributed across independent shards.
// Values are copied in and out; callers storing pointer-like values remain
// responsible for synchronizing access to the pointed-to data.
type Cache[V any] struct {
	state *cacheState[V]

	closeMu sync.Mutex
	cleanup *runtime.Cleanup
	janitor *janitor[V]
}

// New creates a typed cache. A non-positive shard count is normalized to one.
// A zero default expiration means no expiration.
func New[V any](
	defaultExpiration time.Duration,
	cleanupInterval time.Duration,
	shardCount int,
) *Cache[V] {
	if defaultExpiration == DefaultExpiration {
		defaultExpiration = NoExpiration
	}
	if shardCount <= 0 {
		shardCount = 1
	}

	state := &cacheState[V]{
		defaultExpiration: defaultExpiration,
		seed:              randomSeed(),
		shards:            make([]shard[V], shardCount),
	}
	for index := range state.shards {
		state.shards[index].items = make(map[string]Item[V])
	}

	cache := &Cache[V]{state: state}
	if cleanupInterval > 0 {
		janitor := &janitor[V]{
			state:    state,
			interval: cleanupInterval,
			stop:     make(chan struct{}),
			done:     make(chan struct{}),
		}
		go janitor.run()
		cleanup := runtime.AddCleanup(cache, stopJanitor[V], janitor)
		cache.cleanup = &cleanup
		cache.janitor = janitor
	}
	return cache
}

// Close stops background expiration cleanup. It is safe to call repeatedly.
func (cache *Cache[V]) Close() {
	if cache == nil {
		return
	}

	cache.closeMu.Lock()
	cleanup := cache.cleanup
	janitor := cache.janitor
	cache.cleanup = nil
	cache.janitor = nil
	cache.closeMu.Unlock()
	if cleanup != nil {
		cleanup.Stop()
		stopJanitor(janitor)
	}
	runtime.KeepAlive(cache)
}

// Set stores a value, replacing an existing value.
func (cache *Cache[V]) Set(key string, value V, expiration time.Duration) {
	target := cache.shard(key)
	target.mu.Lock()
	target.items[key] = cache.item(value, expiration)
	target.mu.Unlock()
	runtime.KeepAlive(cache)
}

// Add stores a value only when the key is absent or expired.
func (cache *Cache[V]) Add(key string, value V, expiration time.Duration) error {
	target := cache.shard(key)
	now := time.Now().UnixNano()
	target.mu.Lock()
	if current, found := target.items[key]; found && !current.expiredAt(now) {
		target.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrExists, key)
	}
	target.items[key] = cache.item(value, expiration)
	target.mu.Unlock()
	runtime.KeepAlive(cache)
	return nil
}

// Replace stores a value only when the key exists and is not expired.
func (cache *Cache[V]) Replace(key string, value V, expiration time.Duration) error {
	target := cache.shard(key)
	now := time.Now().UnixNano()
	target.mu.Lock()
	if current, found := target.items[key]; !found || current.expiredAt(now) {
		target.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrNotFound, key)
	}
	target.items[key] = cache.item(value, expiration)
	target.mu.Unlock()
	runtime.KeepAlive(cache)
	return nil
}

// Get returns the current value for key.
func (cache *Cache[V]) Get(key string) (V, bool) {
	value, _, found := cache.GetWithExpiration(key)
	return value, found
}

// GetWithExpiration returns the current value and its absolute expiration.
// Values without expiration return a zero time.
func (cache *Cache[V]) GetWithExpiration(key string) (V, time.Time, bool) {
	target := cache.shard(key)
	target.mu.RLock()
	item, found := target.items[key]
	target.mu.RUnlock()
	runtime.KeepAlive(cache)

	if !found || item.Expired() {
		var zero V
		return zero, time.Time{}, false
	}
	if item.Expiration == 0 {
		return item.Value, time.Time{}, true
	}
	return item.Value, time.Unix(0, item.Expiration), true
}

// Update atomically replaces a current value while preserving its expiration.
// The update function runs while the key's shard is locked and must not call
// methods on the same cache.
func (cache *Cache[V]) Update(
	key string,
	update func(V) (V, error),
) (V, error) {
	var zero V
	if update == nil {
		return zero, errors.New("gcache: update function is required")
	}

	target := cache.shard(key)
	now := time.Now().UnixNano()
	target.mu.Lock()
	item, found := target.items[key]
	if !found || item.expiredAt(now) {
		target.mu.Unlock()
		return zero, fmt.Errorf("%w: %q", ErrNotFound, key)
	}
	next, err := update(item.Value)
	if err != nil {
		target.mu.Unlock()
		return zero, err
	}
	item.Value = next
	target.items[key] = item
	target.mu.Unlock()
	runtime.KeepAlive(cache)
	return next, nil
}

// Delete removes key and invokes the eviction callback, when configured.
func (cache *Cache[V]) Delete(key string) {
	target := cache.shard(key)
	target.mu.Lock()
	item, found := target.items[key]
	delete(target.items, key)
	target.mu.Unlock()
	if found {
		cache.notifyEvicted(key, item.Value)
	}
	runtime.KeepAlive(cache)
}

// DeleteExpired removes all expired values and invokes the eviction callback.
func (cache *Cache[V]) DeleteExpired() {
	deleteExpired(cache.state)
	runtime.KeepAlive(cache)
}

// OnEvicted sets the callback used for explicit and expiration-driven deletes.
func (cache *Cache[V]) OnEvicted(callback func(string, V)) {
	cache.state.evictionMu.Lock()
	cache.state.onEvicted = callback
	cache.state.evictionMu.Unlock()
	runtime.KeepAlive(cache)
}

// Items returns a snapshot containing only current values.
func (cache *Cache[V]) Items() map[string]Item[V] {
	now := time.Now().UnixNano()
	items := make(map[string]Item[V])
	for index := range cache.state.shards {
		target := &cache.state.shards[index]
		target.mu.RLock()
		for key, item := range target.items {
			if !item.expiredAt(now) {
				items[key] = item
			}
		}
		target.mu.RUnlock()
	}
	runtime.KeepAlive(cache)
	return items
}

// Len returns the number of current values.
func (cache *Cache[V]) Len() int {
	now := time.Now().UnixNano()
	count := 0
	for index := range cache.state.shards {
		target := &cache.state.shards[index]
		target.mu.RLock()
		for _, item := range target.items {
			if !item.expiredAt(now) {
				count++
			}
		}
		target.mu.RUnlock()
	}
	runtime.KeepAlive(cache)
	return count
}

// Flush deletes every value without invoking the eviction callback.
func (cache *Cache[V]) Flush() {
	for index := range cache.state.shards {
		target := &cache.state.shards[index]
		target.mu.Lock()
		clear(target.items)
		target.mu.Unlock()
	}
	runtime.KeepAlive(cache)
}

// Save writes a snapshot to writer using encoding/gob.
func (cache *Cache[V]) Save(writer io.Writer) error {
	if writer == nil {
		return errors.New("gcache: save writer is required")
	}
	err := gob.NewEncoder(writer).Encode(cache.Items())
	runtime.KeepAlive(cache)
	return err
}

// SaveFile atomically writes a snapshot to filename.
func (cache *Cache[V]) SaveFile(filename string) error {
	temp, err := os.CreateTemp(
		filepath.Dir(filename),
		"."+filepath.Base(filename)+".tmp-*",
	)
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := cache.Save(temp); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, filename)
}

// Load adds unexpired items from reader without replacing current keys.
func (cache *Cache[V]) Load(reader io.Reader) error {
	if reader == nil {
		return errors.New("gcache: load reader is required")
	}
	items := make(map[string]Item[V])
	if err := gob.NewDecoder(reader).Decode(&items); err != nil {
		return err
	}
	now := time.Now().UnixNano()
	for key, item := range items {
		if item.expiredAt(now) {
			continue
		}
		target := cache.shard(key)
		target.mu.Lock()
		if current, found := target.items[key]; !found || current.expiredAt(now) {
			target.items[key] = item
		}
		target.mu.Unlock()
	}
	runtime.KeepAlive(cache)
	return nil
}

// LoadFile adds unexpired items stored in filename.
func (cache *Cache[V]) LoadFile(filename string) (returnErr error) {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	return cache.Load(file)
}

func (cache *Cache[V]) shard(key string) *shard[V] {
	if len(cache.state.shards) == 1 {
		return &cache.state.shards[0]
	}
	index := djb33(cache.state.seed, key) % uint32(len(cache.state.shards))
	return &cache.state.shards[index]
}

func (cache *Cache[V]) item(value V, expiration time.Duration) Item[V] {
	if expiration == DefaultExpiration {
		expiration = cache.state.defaultExpiration
	}
	var deadline int64
	if expiration > 0 {
		deadline = time.Now().Add(expiration).UnixNano()
	}
	return Item[V]{Value: value, Expiration: deadline}
}

func (cache *Cache[V]) notifyEvicted(key string, value V) {
	cache.state.evictionMu.RLock()
	callback := cache.state.onEvicted
	cache.state.evictionMu.RUnlock()
	if callback != nil {
		callback(key, value)
	}
}

func (janitor *janitor[V]) run() {
	ticker := time.NewTicker(janitor.interval)
	defer ticker.Stop()
	defer close(janitor.done)
	for {
		select {
		case <-ticker.C:
			deleteExpired(janitor.state)
		case <-janitor.stop:
			return
		}
	}
}

func stopJanitor[V any](janitor *janitor[V]) {
	if janitor == nil {
		return
	}
	janitor.stopOnce.Do(func() {
		close(janitor.stop)
	})
	<-janitor.done
}

func deleteExpired[V any](state *cacheState[V]) {
	now := time.Now().UnixNano()
	for index := range state.shards {
		target := &state.shards[index]
		var evicted map[string]V
		target.mu.Lock()
		for key, item := range target.items {
			if item.expiredAt(now) {
				if evicted == nil {
					evicted = make(map[string]V)
				}
				evicted[key] = item.Value
				delete(target.items, key)
			}
		}
		target.mu.Unlock()

		state.evictionMu.RLock()
		callback := state.onEvicted
		state.evictionMu.RUnlock()
		if callback != nil {
			for key, value := range evicted {
				callback(key, value)
			}
		}
	}
}

func randomSeed() uint32 {
	var seed [4]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return uint32(time.Now().UnixNano())
	}
	return binary.LittleEndian.Uint32(seed[:])
}

// djb33 is a compact seeded string hash used only for shard selection.
func djb33(seed uint32, key string) uint32 {
	hash := uint32(5381 + seed + uint32(len(key)))
	for index := range len(key) {
		hash = (hash * 33) ^ uint32(key[index])
	}
	return hash ^ (hash >> 16)
}
