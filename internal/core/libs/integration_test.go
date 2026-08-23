package libs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestEmbedLoadEndToEnd: default embedded source + real cache dir under a temp
// XDG_CACHE_HOME loads the bundled library and answers qualified lookups.
func TestEmbedLoadEndToEnd(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cache, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	ld := NewLoader(DefaultSource(), cache)
	idx := symbols.NewIndex()
	if err := ld.load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(idx.LookupQualified("ScalarValues::Real")) != 1 {
		t.Fatal("ScalarValues::Real not indexed via embedded end-to-end load")
	}
}

// TestSysmlLibraryPathOverride: SYSML_LIBRARY_PATH points DefaultSource at an
// on-disk directory, and the loader indexes that custom library instead.
func TestSysmlLibraryPathOverride(t *testing.T) {
	libDir, err := filepath.Abs("testdata/customlib")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYSML_LIBRARY_PATH", libDir)
	cache := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), cache)
	idx := symbols.NewIndex()
	if err := ld.load("Custom.kerml", idx); err != nil {
		t.Fatalf("Load custom: %v", err)
	}
	if len(idx.LookupQualified("Custom::Widget")) != 1 {
		t.Fatal("SYSML_LIBRARY_PATH override did not load Custom::Widget")
	}
	// The bundled library must NOT be visible under the override source.
	if _, err := DefaultSource().Read("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"); err == nil {
		t.Fatal("override source unexpectedly served bundled ScalarValues.kerml")
	}
}

// TestCacheStaleContentReparsedNotServed: a cache entry stored under stale
// content must not be served when the source content differs; the loader
// reparses and produces the correct current symbols.
func TestCacheStaleContentReparsedNotServed(t *testing.T) {
	libDir := t.TempDir()
	libFile := filepath.Join(libDir, "Evolving.kerml")
	if err := os.WriteFile(libFile, []byte("package Evolving { namespace First; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYSML_LIBRARY_PATH", libDir)
	cache := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), cache)

	idx1 := symbols.NewIndex()
	if err := ld.load("Evolving.kerml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if len(idx1.LookupQualified("Evolving::First")) != 1 {
		t.Fatal("first load missing Evolving::First")
	}

	// Change the file content: the old cache key no longer matches, so the
	// loader must reparse and index the NEW member, not serve the stale one.
	if err := os.WriteFile(libFile, []byte("package Evolving { namespace Second; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx2 := symbols.NewIndex()
	if err := ld.load("Evolving.kerml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(idx2.LookupQualified("Evolving::Second")) != 1 {
		t.Fatal("stale cache served: Evolving::Second not indexed after content change")
	}
	if len(idx2.LookupQualified("Evolving::First")) != 0 {
		t.Fatal("stale cache served: Evolving::First should be gone after content change")
	}
}
