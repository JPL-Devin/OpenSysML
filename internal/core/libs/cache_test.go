package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func sampleRecord(name string) *IndexRecord {
	return &IndexRecord{
		Name: name,
		Symbols: []symRecord{
			{FQN: "P", Kind: symbols.SymbolPackage, Span: source.Span{Offset: 0, Len: 1}},
			{FQN: "P::N", Kind: symbols.SymbolNamespace, Span: source.Span{Offset: 2, Len: 3}},
		},
	}
}

func TestCacheStoreLoadRoundTrip(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	rec := sampleRecord("a.kerml")
	key := c.keyFor([]byte("content-a"))
	if err := c.Store(key, rec); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, ok := c.Load(key)
	if !ok {
		t.Fatal("Load miss after Store")
	}
	// symRecord contains a []string field, so it is not ==-comparable;
	// compare the round-tripped fields individually.
	if got.Name != rec.Name || len(got.Symbols) != len(rec.Symbols) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, rec)
	}
	a, b := got.Symbols[1], rec.Symbols[1]
	if a.FQN != b.FQN || a.Kind != b.Kind || a.Span != b.Span {
		t.Fatalf("symbol round-trip mismatch: got %+v want %+v", a, b)
	}
}

func TestCacheLoadUnknownKeyMisses(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	if _, ok := c.Load(c.keyFor([]byte("never-stored"))); ok {
		t.Fatal("Load returned hit for unknown key")
	}
}

func TestCacheKeyDependsOnContentAndVersion(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	k1 := c.keyFor([]byte("alpha"))
	k2 := c.keyFor([]byte("beta"))
	if k1 == k2 {
		t.Fatal("distinct content produced identical cache keys")
	}
	// A record stored under content "alpha" must not be found by content
	// "beta" (stale-content miss) — the core cache-key invariant.
	if err := c.Store(k1, sampleRecord("x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, ok := c.Load(k2); ok {
		t.Fatal("stale content produced a cache hit")
	}
}

func TestNewCacheCreatesDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	if c.dir == "" {
		t.Fatal("NewCache produced empty dir")
	}
	// Store/Load must work against the freshly created dir.
	key := c.keyFor([]byte("z"))
	if err := c.Store(key, sampleRecord("z")); err != nil {
		t.Fatalf("store into new cache dir: %v", err)
	}
	if _, ok := c.Load(key); !ok {
		t.Fatal("Load miss from freshly created cache dir")
	}
}
