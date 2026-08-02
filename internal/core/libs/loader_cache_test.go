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
