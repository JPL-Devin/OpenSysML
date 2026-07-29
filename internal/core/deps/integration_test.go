package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/libs"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestResolveEndToEndDependencyTree(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()

	// Workspace manifest: one local dep "app" + a library-paths dir "shared".
	mkdir := func(p string) string {
		d := filepath.Join(root, p)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		return d
	}
	appDir := mkdir("app")
	libDir := mkdir("shared")
	utilDir := mkdir("util")

	writeFile(t, filepath.Join(appDir, "app.sysml"),
		"package App { namespace Widget; }\n")
	// app transitively depends on util.
	writeFile(t, filepath.Join(appDir, "sysml.toml"),
		"[dependencies.util]\npath = \"../util\"\n")
	writeFile(t, filepath.Join(utilDir, "util.sysml"),
		"package Util { namespace Helper; }\n")
	writeFile(t, filepath.Join(libDir, "shared.kerml"),
		"package Shared { namespace Common; }\n")

	src := `library-paths = ["shared"]

[dependencies.app]
path = "app"
`
	m, err := ParseManifest([]byte(src))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	cache, err := libs.NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	r := NewResolver(fakeFetcher{}, NewLock(), cache)
	idx := symbols.NewIndex()
	if err := r.Resolve(root, m, idx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	for _, fqn := range []string{"App::Widget", "Util::Helper", "Shared::Common"} {
		if got := len(idx.LookupQualified(fqn)); got != 1 {
			t.Fatalf("%s: got %d symbols, want 1", fqn, got)
		}
	}

	// The cache-integrated load path ran: at least one .idx entry persisted.
	entries, err := os.ReadDir(cacheDir(t))
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	var idxFiles int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			idxFiles++
		}
	}
	if idxFiles == 0 {
		t.Fatalf("expected cache to be populated, found no .idx files")
	}
}

// cacheDir returns the libs cache directory under the test's XDG_CACHE_HOME.
func cacheDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.Getenv("XDG_CACHE_HOME"), "sysml-ls", "libs")
}
