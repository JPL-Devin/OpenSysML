// internal/core/deps/resolver_test.go
package deps

import (
	"path/filepath"
	"testing"
)

func TestResolveLocalDependencyDir(t *testing.T) {
	root := t.TempDir()
	geo := filepath.Join(root, "geometry")
	m := &Manifest{Dependencies: map[string]Dep{"geometry": {Path: "geometry"}}}
	r := NewResolver(fakeFetcher{}, NewLock(), nil)
	dirs, err := r.resolveDirs(root, m)
	if err != nil {
		t.Fatalf("resolveDirs: %v", err)
	}
	if dirs["geometry"] != geo {
		t.Fatalf("geometry dir = %q, want %q", dirs["geometry"], geo)
	}
}

func TestResolveGitDependencyRecordsSHA(t *testing.T) {
	root := t.TempDir()
	fake := fakeFetcher{dir: filepath.Join(root, "cached-si"), sha: "abc123"}
	m := &Manifest{Dependencies: map[string]Dep{"si": {Git: "https://x/si.git", Tag: "1.0"}}}
	lock := NewLock()
	r := NewResolver(fake, lock, nil)
	dirs, err := r.resolveDirs(root, m)
	if err != nil {
		t.Fatalf("resolveDirs: %v", err)
	}
	if dirs["si"] != fake.dir {
		t.Fatalf("si dir = %q, want %q", dirs["si"], fake.dir)
	}
	if lock.SHA["si"] != "abc123" {
		t.Fatalf("lock si sha = %q, want abc123", lock.SHA["si"])
	}
}
