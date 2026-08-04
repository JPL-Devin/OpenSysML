package grpc

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestCachePutGet(t *testing.T) {
	cache := NewCache(2) // Max size 2

	hash1 := "abc123"
	model1 := &CachedModel{
		Root:   &ast.RootNamespace{},
		SymTab: symbols.NewScope(nil, nil),
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
	cache := NewCache(2)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	cache := NewCache(2)

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
	cache := NewCache(100)
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
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for maxSize <= 0")
		}
	}()
	NewCache(0)
}
