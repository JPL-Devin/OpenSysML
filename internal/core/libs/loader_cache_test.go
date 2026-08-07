package libs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// countingSource wraps a Source and counts Read calls so we can prove a cache
// hit skips the parse path on the second Load.
type countingSource struct {
	inner Source
	reads int
}

func (c *countingSource) List() []string { return c.inner.List() }
func (c *countingSource) Read(name string) ([]byte, error) {
	c.reads++
	return c.inner.Read(name)
}

func TestLoaderCacheMissThenHit(t *testing.T) {
	cacheDir := t.TempDir()
	cache := &Cache{dir: cacheDir}
	cs := &countingSource{inner: DefaultSource()}
	ld := NewLoader(cs, cache)

	idx1 := symbols.NewIndex()
	if err := ld.Load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if cs.reads != 1 {
		t.Fatalf("reads after first load = %d, want 1", cs.reads)
	}
	if len(idx1.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("first load did not index ScalarValues::Boolean")
	}
	ld.Persist(idx1)
	entries, _ := os.ReadDir(cacheDir)
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			found = true
		}
	}
	if !found {
		t.Fatal("no .idx file written after cache miss")
	}

	idx2 := symbols.NewIndex()
	if err := ld.Load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(idx2.LookupQualified("ScalarValues")) != 1 ||
		len(idx2.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("cached load did not repopulate index")
	}

	// A symbol restored from the cache keeps its specialization targets: it has
	// no Decl, so those edges are the only way its inherited members are found.
	boolean := idx2.LookupQualified("ScalarValues::Boolean")[0]
	if len(boolean.SuperFQNs) != 1 || boolean.SuperFQNs[0] != "ScalarValues::ScalarValue" {
		t.Fatalf("supertypes of the cached Boolean = %v, want [ScalarValues::ScalarValue]", boolean.SuperFQNs)
	}
}

// A record whose supertypes are not all reachable yet must not be cached when
// the loader requires resolution: its key is the content alone, so it would be
// restored — minus that edge — in a context where the target is present.
func TestLoaderRequireResolvedSkipsUnresolvedRecord(t *testing.T) {
	dir := t.TempDir()
	// Specializes ScalarValues::Real, which this directory does not declare.
	src := filepath.Join(dir, "lib.sysml")
	if err := os.WriteFile(src, []byte("package Lib { attribute def Mass :> ScalarValues::Real; }"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()
	ld := NewLoader(NewDirSource(dir), &Cache{dir: cacheDir})
	ld.RequireResolved = true

	idx := symbols.NewIndex()
	if err := ld.Load("lib.sysml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ld.Persist(idx)

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			t.Fatalf("cached %s despite the unresolved supertype ScalarValues::Real", e.Name())
		}
	}
}

func TestIndexAddRecordsRemovable(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddRecords("lib.kerml", []symbols.RecordEntry{
		{FQN: "P", Kind: symbols.SymbolPackage},
		{FQN: "P::N", Kind: symbols.SymbolNamespace},
	})
	if len(idx.LookupQualified("P::N")) != 1 {
		t.Fatal("AddRecords did not register P::N")
	}
	idx.RemoveDocument("lib.kerml")
	if len(idx.LookupQualified("P::N")) != 0 {
		t.Fatal("RemoveDocument did not drop record-added symbols")
	}
}
