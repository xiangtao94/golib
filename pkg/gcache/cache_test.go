package gcache

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestCacheKeepsValuesTyped(t *testing.T) {
	cache := New[string](time.Minute, 0, 8)
	cache.Set("answer", "forty-two", DefaultExpiration)

	value, found := cache.Get("answer")

	if !found || value != "forty-two" {
		t.Fatalf("Get() = %q, %v", value, found)
	}
}

func TestCacheAddReplaceAndUpdate(t *testing.T) {
	cache := New[int](NoExpiration, 0, 4)
	if err := cache.Add("count", 1, DefaultExpiration); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := cache.Add("count", 2, DefaultExpiration); !errors.Is(err, ErrExists) {
		t.Fatalf("Add() error = %v, want ErrExists", err)
	}
	if err := cache.Replace("missing", 2, DefaultExpiration); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Replace() error = %v, want ErrNotFound", err)
	}

	value, err := cache.Update("count", func(value int) (int, error) {
		return value + 41, nil
	})
	if err != nil || value != 42 {
		t.Fatalf("Update() = %d, %v", value, err)
	}
	if value, found := cache.Get("count"); !found || value != 42 {
		t.Fatalf("Get() = %d, %v", value, found)
	}
}

func TestCacheUpdateIsAtomic(t *testing.T) {
	cache := New[int](NoExpiration, 0, 16)
	cache.Set("count", 0, DefaultExpiration)

	const workers = 20
	const increments = 100
	var wait sync.WaitGroup
	for range workers {
		wait.Go(func() {
			for range increments {
				if _, err := cache.Update("count", func(value int) (int, error) {
					return value + 1, nil
				}); err != nil {
					t.Errorf("Update() error = %v", err)
					return
				}
			}
		})
	}
	wait.Wait()

	value, _ := cache.Get("count")
	if value != workers*increments {
		t.Fatalf("count = %d, want %d", value, workers*increments)
	}
}

func TestCacheExpirationAndJanitor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := New[string](time.Minute, 10*time.Second, 2)
		t.Cleanup(cache.Close)
		evicted := make(chan string, 1)
		cache.OnEvicted(func(key, _ string) {
			evicted <- key
		})
		cache.Set("short", "value", 5*time.Second)

		time.Sleep(10 * time.Second)
		synctest.Wait()

		if _, found := cache.Get("short"); found {
			t.Fatal("expired value is still present")
		}
		select {
		case key := <-evicted:
			if key != "short" {
				t.Fatalf("evicted key = %q", key)
			}
		default:
			t.Fatal("janitor did not invoke eviction callback")
		}
	})
}

func TestCacheGetDeletesExpiredValueWithoutJanitor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := New[string](NoExpiration, 0, 1)
		evicted := make(chan string, 1)
		cache.OnEvicted(func(key, _ string) {
			evicted <- key
		})
		cache.Set("expired", "value", time.Second)

		time.Sleep(2 * time.Second)
		if _, found := cache.Get("expired"); found {
			t.Fatal("Get() found an expired value")
		}

		target := cache.shard("expired")
		target.mu.RLock()
		_, retained := target.items["expired"]
		target.mu.RUnlock()
		if retained {
			t.Fatal("Get() retained the expired value")
		}
		if key := <-evicted; key != "expired" {
			t.Fatalf("evicted key = %q, want expired", key)
		}
	})
}

func TestCacheBoundsEntries(t *testing.T) {
	cache := NewWithMaxEntries[int](NoExpiration, 0, 1, 2)
	cache.Set("first", 1, DefaultExpiration)
	cache.Set("second", 2, DefaultExpiration)
	cache.Set("third", 3, DefaultExpiration)

	if got := cache.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
}

func TestCacheUsesFullGlobalCapacityAcrossShards(t *testing.T) {
	const capacity = 100
	cache := NewWithMaxEntries[int](NoExpiration, 0, 16, capacity)
	for index := range capacity {
		cache.Set(string(rune(index)), index, DefaultExpiration)
	}

	if got := cache.Len(); got != capacity {
		t.Fatalf("Len() = %d, want full global capacity %d", got, capacity)
	}
}

func TestCacheGlobalCapacityRemainsBoundedUnderConcurrentWrites(t *testing.T) {
	const capacity = 100
	cache := NewWithMaxEntries[int](NoExpiration, 0, 16, capacity)

	var writers sync.WaitGroup
	for writer := range 32 {
		writers.Go(func() {
			for item := range 1_000 {
				key := fmt.Sprintf("%d:%d", writer, item)
				cache.Set(key, item, DefaultExpiration)
			}
		})
	}
	writers.Wait()

	if got := cache.Len(); got > capacity {
		t.Fatalf("Len() = %d, want at most %d", got, capacity)
	}
	if got := cache.state.entryCount.Load(); got > capacity {
		t.Fatalf("retained entry count = %d, want at most %d", got, capacity)
	}
}

func TestCacheItemsAreSnapshotAndFlushClears(t *testing.T) {
	cache := New[string](NoExpiration, 0, 3)
	cache.Set("a", "one", DefaultExpiration)
	items := cache.Items()
	items["a"] = Item[string]{Value: "changed"}

	value, _ := cache.Get("a")
	if value != "one" {
		t.Fatalf("snapshot mutation changed cache value to %q", value)
	}
	if cache.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", cache.Len())
	}
	target := cache.shard("a")
	target.mu.RLock()
	previousItems := target.items
	target.mu.RUnlock()
	cache.Flush()
	if cache.Len() != 0 {
		t.Fatalf("Len() after Flush = %d", cache.Len())
	}
	if len(previousItems) != 1 {
		t.Fatal("Flush() cleared the old map instead of releasing its buckets")
	}
}

func TestCacheLenDoesNotAllocateAFullSnapshot(t *testing.T) {
	cache := New[int](NoExpiration, 0, 4)
	for index := range 1_000 {
		cache.Set(string(rune(index)), index, DefaultExpiration)
	}

	allocations := testing.AllocsPerRun(100, func() {
		if got := cache.Len(); got != 1_000 {
			t.Fatalf("Len() = %d, want 1000", got)
		}
	})
	if allocations != 0 {
		t.Fatalf("Len() allocations = %v, want 0", allocations)
	}
}

func TestCacheSaveLoadRoundTrip(t *testing.T) {
	source := New[string](NoExpiration, 0, 2)
	source.Set("preserved", "from-source", DefaultExpiration)
	source.Set("new", "loaded", DefaultExpiration)

	var encoded bytes.Buffer
	if err := source.Save(&encoded); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	destination := New[string](NoExpiration, 0, 4)
	destination.Set("preserved", "from-destination", DefaultExpiration)
	if err := destination.Load(&encoded); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if value, _ := destination.Get("preserved"); value != "from-destination" {
		t.Fatalf("existing value overwritten with %q", value)
	}
	if value, found := destination.Get("new"); !found || value != "loaded" {
		t.Fatalf("loaded value = %q, %v", value, found)
	}
}

func TestCacheSaveFileLoadFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.gob")
	source := New[string](NoExpiration, 0, 2)
	source.Set("saved", "value", DefaultExpiration)

	if err := source.SaveFile(path); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	destination := New[string](NoExpiration, 0, 2)
	if err := destination.LoadFile(path); err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if value, found := destination.Get("saved"); !found || value != "value" {
		t.Fatalf("loaded value = %q, %v", value, found)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("persistence files = %v, want only %q", entries, filepath.Base(path))
	}
}

func TestCacheSaveFileFailureDoesNotReplaceDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.gob")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cache := New[any](NoExpiration, 0, 1)
	cache.Set("unsupported", func() {}, DefaultExpiration)

	if err := cache.SaveFile(path); err == nil {
		t.Fatal("SaveFile() error = nil, want gob encoding error")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "original" {
		t.Fatalf("destination content = %q, want original", content)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("persistence files after failure = %v", entries)
	}
}

func TestCacheCloseIsIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := New[string](time.Minute, time.Hour, 1)
		janitor := cache.janitor
		if cache.cleanup == nil || janitor == nil {
			t.Fatal("New() did not install cleanup ownership")
		}
		cache.Close()
		cache.Close()
		if cache.cleanup != nil || cache.janitor != nil {
			t.Fatal("Close() retained cleanup ownership")
		}
		select {
		case <-janitor.done:
		default:
			t.Fatal("Close() returned before janitor stopped")
		}
	})
}

func TestDjb33UsesEveryKeyByte(t *testing.T) {
	const seed = 42
	if djb33(seed, "a") == djb33(seed, "b") {
		t.Fatal("single-byte keys must not hash to the same value")
	}
	if djb33(seed, "prefix-a") == djb33(seed, "prefix-b") {
		t.Fatal("the final key byte must affect the hash")
	}
}
