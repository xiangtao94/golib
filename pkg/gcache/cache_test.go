package gcache

import (
	"bytes"
	"errors"
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
	cache.Flush()
	if cache.Len() != 0 {
		t.Fatalf("Len() after Flush = %d", cache.Len())
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

func TestCacheCloseIsIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := New[string](time.Minute, time.Hour, 1)
		cache.Close()
		cache.Close()
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
