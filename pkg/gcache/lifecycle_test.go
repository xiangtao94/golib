package gcache

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBucketCacheCloseIsImmediateAndIdempotent(t *testing.T) {
	cache := NewBucketCache(time.Minute, time.Hour, 1)
	done := make(chan struct{})
	go func() {
		cache.Close()
		cache.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close blocked")
	}
}

func TestSaveFileReturnsSerializationErrors(t *testing.T) {
	cache := NewBucketCache(time.Minute, 0, 1)
	cache.Set("unsupported", func() {}, NoExpiration)

	err := cache.SaveFile(filepath.Join(t.TempDir(), "cache"))

	if err == nil {
		t.Fatal("SaveFile must return gob serialization errors")
	}
}
