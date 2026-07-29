// Package gcache provides a typed, sharded in-memory cache.
package gcache

import (
	"bytes"
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
	"sync/atomic"
	"time"
)

const (
	// NoExpiration keeps an item until it is explicitly deleted.
	NoExpiration time.Duration = -1
	// DefaultExpiration uses the duration configured when the cache was created.
	DefaultExpiration time.Duration = 0
	// DefaultMaxEntries bounds the number of values retained by a cache.
	DefaultMaxEntries = 100_000
	// DefaultMaxSnapshotBytes is the largest encoded snapshot accepted by
	// default-capacity caches. Smaller caches use a proportionally smaller
	// limit with a 1 MiB floor.
	DefaultMaxSnapshotBytes int64 = 64 << 20

	minSnapshotBytes             int64 = 1 << 20
	defaultSnapshotBytesPerEntry int64 = 1 << 10
	maxLegacySnapshotBytes       int64 = 4 << 20
	snapshotMagic                      = "gcache\x00\x02"
)

var (
	ErrNotFound         = errors.New("gcache: item not found")
	ErrExists           = errors.New("gcache: item already exists")
	ErrSnapshotTooLarge = errors.New("gcache: snapshot exceeds load limits")
)

// Item is a serializable cache entry.
type Item[V any] struct {
	Value      V
	Expiration int64
}

type snapshotEntry[V any] struct {
	Key  string
	Item Item[V]
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
	maxEntries        int64
	entryCount        atomic.Int64

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
	janitor *janitor[V]
	cleanup *runtime.Cleanup
}

// New creates a typed cache. A non-positive shard count is normalized to one.
// A zero default expiration means no expiration. When cleanupInterval is
// positive, the caller must call Close when the cache is no longer needed.
func New[V any](
	defaultExpiration time.Duration,
	cleanupInterval time.Duration,
	shardCount int,
) *Cache[V] {
	return NewWithMaxEntries[V](
		defaultExpiration,
		cleanupInterval,
		shardCount,
		DefaultMaxEntries,
	)
}

// NewWithMaxEntries creates a cache with an explicit upper bound. A
// non-positive maxEntries value uses DefaultMaxEntries. When cleanupInterval is
// positive, the caller must call Close when the cache is no longer needed.
func NewWithMaxEntries[V any](
	defaultExpiration time.Duration,
	cleanupInterval time.Duration,
	shardCount int,
	maxEntries int,
) *Cache[V] {
	if defaultExpiration == DefaultExpiration {
		defaultExpiration = NoExpiration
	}
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	if shardCount <= 0 {
		shardCount = 1
	}
	if shardCount > maxEntries {
		shardCount = maxEntries
	}

	state := &cacheState[V]{
		defaultExpiration: defaultExpiration,
		seed:              randomSeed(),
		shards:            make([]shard[V], shardCount),
		maxEntries:        int64(maxEntries),
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
		cache.janitor = janitor
		cleanup := runtime.AddCleanup(cache, requestStopJanitor[V], janitor)
		cache.cleanup = &cleanup
	}
	return cache
}

// RequestClose asks background expiration cleanup to stop without waiting.
// It is safe to call from an eviction callback.
func (cache *Cache[V]) RequestClose() {
	if cache == nil {
		return
	}

	cache.closeMu.Lock()
	janitor := cache.janitor
	cache.closeMu.Unlock()
	requestStopJanitor(janitor)
}

// Close stops background expiration cleanup and waits for it to finish. It is
// safe to call repeatedly. Eviction callbacks must use RequestClose instead.
func (cache *Cache[V]) Close() {
	if cache == nil {
		return
	}

	cache.closeMu.Lock()
	janitor := cache.janitor
	cleanup := cache.cleanup
	cache.cleanup = nil
	cache.closeMu.Unlock()
	if cleanup != nil {
		cleanup.Stop()
	}
	requestStopJanitor(janitor)
	if janitor == nil {
		runtime.KeepAlive(cache)
		return
	}
	<-janitor.done

	cache.closeMu.Lock()
	if cache.janitor == janitor {
		cache.janitor = nil
	}
	cache.closeMu.Unlock()
	runtime.KeepAlive(cache)
}

// Set stores a value, replacing an existing value.
func (cache *Cache[V]) Set(key string, value V, expiration time.Duration) {
	target := cache.shard(key)
	target.mu.Lock()
	evictedKey, evictedValue, evicted := storeItem(
		cache.state,
		target,
		key,
		cache.item(value, expiration),
	)
	target.mu.Unlock()
	if evicted {
		cache.notifyEvicted(evictedKey, evictedValue)
	}
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
	evictedKey, evictedValue, evicted := storeItem(
		cache.state,
		target,
		key,
		cache.item(value, expiration),
	)
	target.mu.Unlock()
	if evicted {
		cache.notifyEvicted(evictedKey, evictedValue)
	}
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
	if !found {
		var zero V
		return zero, time.Time{}, false
	}
	now := time.Now().UnixNano()
	if item.expiredAt(now) {
		target.mu.Lock()
		item, found = target.items[key]
		if found && item.expiredAt(now) {
			delete(target.items, key)
			cache.state.entryCount.Add(-1)
			target.mu.Unlock()
			cache.notifyEvicted(key, item.Value)
			var zero V
			return zero, time.Time{}, false
		}
		target.mu.Unlock()
		if !found {
			var zero V
			return zero, time.Time{}, false
		}
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
	return next, nil
}

// Delete removes key and invokes the eviction callback, when configured.
func (cache *Cache[V]) Delete(key string) {
	target := cache.shard(key)
	target.mu.Lock()
	item, found := target.items[key]
	delete(target.items, key)
	if found {
		cache.state.entryCount.Add(-1)
	}
	target.mu.Unlock()
	if found {
		cache.notifyEvicted(key, item.Value)
	}
}

// DeleteExpired removes all expired values and invokes the eviction callback.
func (cache *Cache[V]) DeleteExpired() {
	deleteExpired(cache.state)
}

// OnEvicted sets the callback used for explicit and expiration-driven deletes.
func (cache *Cache[V]) OnEvicted(callback func(string, V)) {
	cache.state.evictionMu.Lock()
	cache.state.onEvicted = callback
	cache.state.evictionMu.Unlock()
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
	return count
}

// Flush deletes every value without invoking the eviction callback.
func (cache *Cache[V]) Flush() {
	for index := range cache.state.shards {
		target := &cache.state.shards[index]
		target.mu.Lock()
		removed := len(target.items)
		target.items = make(map[string]Item[V])
		cache.state.entryCount.Add(-int64(removed))
		target.mu.Unlock()
	}
}

// Save writes a bounded-entry snapshot to writer using encoding/gob.
func (cache *Cache[V]) Save(writer io.Writer) error {
	if writer == nil {
		return errors.New("gcache: save writer is required")
	}
	items := cache.Items()
	written, err := io.WriteString(writer, snapshotMagic)
	if err != nil {
		return err
	}
	if written != len(snapshotMagic) {
		return io.ErrShortWrite
	}
	if err := binary.Write(writer, binary.BigEndian, uint64(len(items))); err != nil {
		return err
	}

	encoder := gob.NewEncoder(writer)
	for key, item := range items {
		if err := encoder.Encode(snapshotEntry[V]{Key: key, Item: item}); err != nil {
			return err
		}
	}
	return nil
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
	return cache.LoadWithLimit(reader, cache.snapshotLoadLimit())
}

// LoadWithLimit adds unexpired items from a size-bounded encoded snapshot.
// The snapshot must also contain no more entries than this cache can retain.
func (cache *Cache[V]) LoadWithLimit(reader io.Reader, maxBytes int64) error {
	if reader == nil {
		return errors.New("gcache: load reader is required")
	}
	if maxBytes <= 0 {
		return errors.New("gcache: snapshot byte limit must be positive")
	}
	encoded, err := readSnapshot(reader, maxBytes)
	if err != nil {
		return err
	}

	var items map[string]Item[V]
	if bytes.HasPrefix(encoded, []byte(snapshotMagic)) {
		items, err = cache.decodeSnapshot(encoded[len(snapshotMagic):])
	} else {
		items, err = cache.decodeLegacySnapshot(encoded)
	}
	if err != nil {
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
			evictedKey, evictedValue, evicted := storeItem(
				cache.state,
				target,
				key,
				item,
			)
			target.mu.Unlock()
			if evicted {
				cache.notifyEvicted(evictedKey, evictedValue)
			}
			continue
		}
		target.mu.Unlock()
	}
	return nil
}

func (cache *Cache[V]) decodeSnapshot(encoded []byte) (map[string]Item[V], error) {
	reader := bytes.NewReader(encoded)
	var count uint64
	if err := binary.Read(reader, binary.BigEndian, &count); err != nil {
		return nil, err
	}
	if count > uint64(cache.state.maxEntries) {
		return nil, fmt.Errorf(
			"%w: declares %d entries, cache capacity is %d",
			ErrSnapshotTooLarge,
			count,
			cache.state.maxEntries,
		)
	}

	items := make(map[string]Item[V], int(count))
	decoder := gob.NewDecoder(reader)
	for range count {
		var entry snapshotEntry[V]
		if err := decoder.Decode(&entry); err != nil {
			return nil, err
		}
		if _, exists := items[entry.Key]; exists {
			return nil, fmt.Errorf("gcache: duplicate snapshot key %q", entry.Key)
		}
		items[entry.Key] = entry.Item
	}

	var extra snapshotEntry[V]
	switch err := decoder.Decode(&extra); {
	case errors.Is(err, io.EOF):
		return items, nil
	case err != nil:
		return nil, err
	default:
		return nil, errors.New("gcache: snapshot contains undeclared entries")
	}
}

func (cache *Cache[V]) decodeLegacySnapshot(encoded []byte) (map[string]Item[V], error) {
	if int64(len(encoded)) > maxLegacySnapshotBytes {
		return nil, fmt.Errorf(
			"%w: legacy encoded size %d exceeds compatibility limit %d",
			ErrSnapshotTooLarge,
			len(encoded),
			maxLegacySnapshotBytes,
		)
	}

	items := make(map[string]Item[V], min(int(cache.state.maxEntries), 1_024))
	if err := gob.NewDecoder(bytes.NewReader(encoded)).Decode(&items); err != nil {
		return nil, err
	}
	if int64(len(items)) > cache.state.maxEntries {
		return nil, fmt.Errorf(
			"%w: contains %d entries, cache capacity is %d",
			ErrSnapshotTooLarge,
			len(items),
			cache.state.maxEntries,
		)
	}
	return items, nil
}

func (cache *Cache[V]) snapshotLoadLimit() int64 {
	if cache.state.maxEntries >=
		DefaultMaxSnapshotBytes/defaultSnapshotBytesPerEntry {
		return DefaultMaxSnapshotBytes
	}
	limit := cache.state.maxEntries * defaultSnapshotBytesPerEntry
	if limit < minSnapshotBytes {
		return minSnapshotBytes
	}
	return limit
}

func readSnapshot(reader io.Reader, maxBytes int64) ([]byte, error) {
	if remaining, ok := reader.(interface{ Len() int }); ok &&
		int64(remaining.Len()) > maxBytes {
		return nil, fmt.Errorf(
			"%w: encoded size %d exceeds byte limit %d",
			ErrSnapshotTooLarge,
			remaining.Len(),
			maxBytes,
		)
	}

	limited := &io.LimitedReader{R: reader, N: maxBytes}
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if limited.N == 0 {
		var extra [1]byte
		read, readErr := io.ReadFull(reader, extra[:])
		if read > 0 {
			return nil, fmt.Errorf(
				"%w: encoded size exceeds byte limit %d",
				ErrSnapshotTooLarge,
				maxBytes,
			)
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, readErr
		}
	}
	return encoded, nil
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

func storeItem[V any](
	state *cacheState[V],
	target *shard[V],
	key string,
	item Item[V],
) (string, V, bool) {
	var zero V
	_, found := target.items[key]
	target.items[key] = item
	if found || state.entryCount.Add(1) <= state.maxEntries {
		return "", zero, false
	}
	for evictedKey, item := range target.items {
		delete(target.items, evictedKey)
		state.entryCount.Add(-1)
		return evictedKey, item.Value, true
	}
	state.entryCount.Add(-1)
	return "", zero, false
}

func (janitor *janitor[V]) run() {
	ticker := time.NewTicker(janitor.interval)
	defer ticker.Stop()
	defer close(janitor.done)
	for {
		select {
		case <-ticker.C:
			janitor.deleteExpired()
		case <-janitor.stop:
			return
		}
	}
}

func (janitor *janitor[V]) deleteExpired() {
	now := time.Now().UnixNano()
	for index := range janitor.state.shards {
		select {
		case <-janitor.stop:
			return
		default:
		}
		evicted := deleteExpiredFromShard(
			janitor.state,
			&janitor.state.shards[index],
			now,
		)
		notifyEvicted(janitor.state, evicted)
	}
}

func requestStopJanitor[V any](janitor *janitor[V]) {
	if janitor == nil {
		return
	}
	janitor.stopOnce.Do(func() {
		close(janitor.stop)
	})
}

func deleteExpired[V any](state *cacheState[V]) {
	now := time.Now().UnixNano()
	for index := range state.shards {
		evicted := deleteExpiredFromShard(state, &state.shards[index], now)
		notifyEvicted(state, evicted)
	}
}

func notifyEvicted[V any](state *cacheState[V], evicted map[string]V) {
	if len(evicted) == 0 {
		return
	}
	state.evictionMu.RLock()
	callback := state.onEvicted
	state.evictionMu.RUnlock()
	if callback == nil {
		return
	}
	for key, value := range evicted {
		callback(key, value)
	}
}

func deleteExpiredFromShard[V any](
	state *cacheState[V],
	target *shard[V],
	now int64,
) map[string]V {
	var evicted map[string]V
	target.mu.Lock()
	for key, item := range target.items {
		if item.expiredAt(now) {
			if evicted == nil {
				evicted = make(map[string]V)
			}
			evicted[key] = item.Value
			delete(target.items, key)
			state.entryCount.Add(-1)
		}
	}
	target.mu.Unlock()
	return evicted
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
