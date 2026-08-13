package grpc

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// CachedModel holds parsed model data with semantic analysis results
type CachedModel struct {
	Root        *ast.RootNamespace
	Index       *symbols.Index      // For symbol lookups by FQN
	Source      *source.SourceFile  // For diagnostic line/col mapping
	ParseDiags  []parser.Diagnostic // Parser diagnostics
	PassesDiags []passes.Diagnostic // Semantic pass diagnostics (name-resolution, type, constraint)

	symCtxOnce sync.Once
	symCtx     *SymbolContext
}

// SymbolContext returns the conversion context for this model, building it on
// first use. Name resolution and the semantic relations derived from it are
// memoized in it, so every symbol converted from one cached model shares one.
func (m *CachedModel) SymbolContext() *SymbolContext {
	m.symCtxOnce.Do(func() {
		m.symCtx = NewSymbolContext(m.Index)
	})
	return m.symCtx
}

// Cache is an LRU cache for parsed models keyed by content hash
type Cache struct {
	mu      sync.RWMutex
	maxSize int
	items   map[string]*list.Element
	lruList *list.List
}

type cacheEntry struct {
	key   string
	value *CachedModel
}

// NewCache creates a cache with the specified max size. It returns an error if
// maxSize is not positive.
func NewCache(maxSize int) (*Cache, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("cache maxSize must be positive, got %d", maxSize)
	}
	return &Cache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		lruList: list.New(),
	}, nil
}

// Get retrieves a model from cache, returns (model, true) on hit
func (c *Cache) Get(hash string) (*CachedModel, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[hash]
	if !ok {
		return nil, false
	}

	c.lruList.MoveToFront(elem)
	entry := elem.Value.(*cacheEntry)
	return entry.value, true
}

// Put adds a model to cache, evicting LRU if at capacity
func (c *Cache) Put(hash string, model *CachedModel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if elem, ok := c.items[hash]; ok {
		c.lruList.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = model
		return
	}

	// Evict if at capacity
	if c.lruList.Len() >= c.maxSize {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	// Add new entry
	entry := &cacheEntry{key: hash, value: model}
	elem := c.lruList.PushFront(entry)
	c.items[hash] = elem
}
