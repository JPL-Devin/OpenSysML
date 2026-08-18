package grpc

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// mustNewCache builds a cache, failing the test if construction errors.
func mustNewCache(t *testing.T, maxSize int) *Cache {
	t.Helper()
	cache, err := NewCache(maxSize)
	if err != nil {
		t.Fatalf("NewCache(%d): %v", maxSize, err)
	}
	return cache
}

// mustNewService builds a service, failing the test if construction errors.
func mustNewService(t *testing.T, cacheSize int) *Service {
	t.Helper()
	srv, err := NewService(cacheSize, "test")
	if err != nil {
		t.Fatalf("NewService(%d): %v", cacheSize, err)
	}
	return srv
}

func TestCachePutGet(t *testing.T) {
	cache := mustNewCache(t, 2) // Max size 2

	hash1 := "abc123"
	model1 := &CachedModel{
		Root:  &ast.RootNamespace{},
		Index: symbols.NewIndex(),
	}

	cache.Put(hash1, model1)

	got, ok := cache.Get(hash1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != model1 {
		t.Error("got different model")
	}
}

func TestCacheMiss(t *testing.T) {
	cache := mustNewCache(t, 2)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	cache := mustNewCache(t, 2)

	model1 := &CachedModel{Root: &ast.RootNamespace{}}
	model2 := &CachedModel{Root: &ast.RootNamespace{}}
	model3 := &CachedModel{Root: &ast.RootNamespace{}}

	cache.Put("hash1", model1)
	cache.Put("hash2", model2)
	cache.Put("hash3", model3) // Should evict hash1

	_, ok := cache.Get("hash1")
	if ok {
		t.Error("expected hash1 to be evicted")
	}

	_, ok = cache.Get("hash2")
	if !ok {
		t.Error("expected hash2 to still be cached")
	}

	_, ok = cache.Get("hash3")
	if !ok {
		t.Error("expected hash3 to be cached")
	}
}

func TestCacheThreadSafety(t *testing.T) {
	cache := mustNewCache(t, 100)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			model := &CachedModel{Root: &ast.RootNamespace{}}
			cache.Put(fmt.Sprintf("key%d", n), model)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cache.Get(fmt.Sprintf("key%d", n))
		}(i)
	}

	wg.Wait()
	// If we reach here without race detector firing, thread safety works
}

func TestCacheInvalidMaxSize(t *testing.T) {
	if _, err := NewCache(0); err == nil {
		t.Error("expected error for maxSize <= 0")
	}
	if _, err := NewService(0, "test"); err == nil {
		t.Error("expected error for cacheSize <= 0")
	}
}
