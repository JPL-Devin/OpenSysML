// internal/core/deps/resolver_test.go
package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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

func TestResolveTransitiveDependency(t *testing.T) {
	root := t.TempDir()

	// Workspace depends on "a"; a depends on "b" (nested sysml.toml).
	aDir := filepath.Join(root, "a")
	bDir := filepath.Join(root, "b")
	if err := os.MkdirAll(aDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(aDir, "a.sysml"),
		"package A { namespace AThing; }\n")
	// a's own manifest depends on b via a path relative to a's dir.
	writeFile(t, filepath.Join(aDir, "sysml.toml"),
		"[dependencies.b]\npath = \"../b\"\n")
	writeFile(t, filepath.Join(bDir, "b.sysml"),
		"package B { namespace BThing; }\n")

	m := &Manifest{Dependencies: map[string]Dep{"a": {Path: "a"}}}
	r := NewResolver(fakeFetcher{}, NewLock(), nil)
	idx := symbols.NewIndex()
	if err := r.Resolve(root, m, idx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := len(idx.LookupQualified("A::AThing")); got != 1 {
		t.Fatalf("A::AThing: got %d, want 1", got)
	}
	if got := len(idx.LookupQualified("B::BThing")); got != 1 {
		t.Fatalf("B::BThing (transitive): got %d, want 1", got)
	}
}

func TestResolveDedupSharedGitDependency(t *testing.T) {
	// Two deps resolve (via fake) to the SAME dir + SHA; the shared dir's
	// files must be loaded exactly once (no duplicate LookupQualified hit).
	root := t.TempDir()
	shared := filepath.Join(root, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(shared, "s.sysml"),
		"package S { namespace SThing; }\n")

	fake := fakeFetcher{dir: shared, sha: "samesha"}
	m := &Manifest{Dependencies: map[string]Dep{
		"one": {Git: "https://x/one.git", Rev: "r1"},
		"two": {Git: "https://x/two.git", Rev: "r2"},
	}}
	r := NewResolver(fake, NewLock(), nil)
	idx := symbols.NewIndex()
	if err := r.Resolve(root, m, idx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := len(idx.LookupQualified("S::SThing")); got != 1 {
		t.Fatalf("S::SThing: got %d, want 1 (loaded once despite two deps)", got)
	}
}

func TestResolveCycleTerminates(t *testing.T) {
	// a -> b -> a (local path cycle). Must terminate and load both once.
	root := t.TempDir()
	aDir := filepath.Join(root, "a")
	bDir := filepath.Join(root, "b")
	if err := os.MkdirAll(aDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(aDir, "a.sysml"),
		"package A { namespace AThing; }\n")
	writeFile(t, filepath.Join(aDir, "sysml.toml"),
		"[dependencies.b]\npath = \"../b\"\n")
	writeFile(t, filepath.Join(bDir, "b.sysml"),
		"package B { namespace BThing; }\n")
	writeFile(t, filepath.Join(bDir, "sysml.toml"),
		"[dependencies.a]\npath = \"../a\"\n")

	m := &Manifest{Dependencies: map[string]Dep{"a": {Path: "a"}}}
	r := NewResolver(fakeFetcher{}, NewLock(), nil)
	idx := symbols.NewIndex()
	if err := r.Resolve(root, m, idx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := len(idx.LookupQualified("A::AThing")); got != 1 {
		t.Fatalf("A::AThing: got %d, want 1", got)
	}
	if got := len(idx.LookupQualified("B::BThing")); got != 1 {
		t.Fatalf("B::BThing: got %d, want 1", got)
	}
}
