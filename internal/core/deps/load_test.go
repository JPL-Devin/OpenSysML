package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestLoadDirRegistersDependencyFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "geometry.sysml"),
		"package Geometry { namespace Circle; }\n")

	idx := symbols.NewIndex()
	if err := loadDir(dir, idx, nil); err != nil {
		t.Fatalf("loadDir: %v", err)
	}
	if got := len(idx.LookupQualified("Geometry::Circle")); got != 1 {
		t.Fatalf("Geometry::Circle: got %d symbols, want 1", got)
	}
}

func TestResolveLoadsLocalDependencyIntoIndex(t *testing.T) {
	root := t.TempDir()
	depDir := filepath.Join(root, "geometry")
	if err := os.MkdirAll(depDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(depDir, "geometry.sysml"),
		"package Geometry { namespace Circle; }\n")

	m := &Manifest{Dependencies: map[string]Dep{"geometry": {Path: "geometry"}}}
	r := NewResolver(fakeFetcher{}, NewLock(), nil)
	idx := symbols.NewIndex()
	if err := r.Resolve(root, m, idx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := len(idx.LookupQualified("Geometry::Circle")); got != 1 {
		t.Fatalf("Geometry::Circle: got %d symbols, want 1", got)
	}
}

func TestResolveLoadsLibraryPathDir(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "extra")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(libDir, "extra.kerml"),
		"package Extra { namespace Widget; }\n")

	m := &Manifest{LibraryPaths: []string{"extra"}}
	r := NewResolver(fakeFetcher{}, NewLock(), nil)
	idx := symbols.NewIndex()
	if err := r.Resolve(root, m, idx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := len(idx.LookupQualified("Extra::Widget")); got != 1 {
		t.Fatalf("Extra::Widget: got %d symbols, want 1", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
