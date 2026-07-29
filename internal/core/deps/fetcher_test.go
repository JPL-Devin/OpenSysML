package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheDirForGit(t *testing.T) {
	f := &gitFetcher{cacheDir: "/cache"}
	dep := Dep{Git: "https://github.com/acme/geometry.git", Rev: "abc123"}
	got := f.cacheDirFor("geometry", dep)
	want := filepath.Join("/cache", "github.com", "acme", "geometry", "abc123")
	if got != want {
		t.Fatalf("cacheDirFor = %q, want %q", got, want)
	}
}

func TestGitFetcherUsesCachedCheckout(t *testing.T) {
	// If the target dir already exists with content, gitFetcher must NOT
	// re-clone; it returns the cached dir + the pinned rev as sha.
	cache := t.TempDir()
	f := &gitFetcher{cacheDir: cache}
	dep := Dep{Git: "https://example.com/x.git", Rev: "deadbeef"}
	target := f.cacheDirFor("x", dep)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "a.sysml"), []byte("package X;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, sha, err := f.Fetch("x", dep)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if dir != target {
		t.Errorf("dir = %q, want %q", dir, target)
	}
	if sha != "deadbeef" {
		t.Errorf("sha = %q, want deadbeef", sha)
	}
}

func TestFakeFetcher(t *testing.T) {
	fixture := t.TempDir()
	fake := fakeFetcher{dir: fixture, sha: "cafef00d"}
	dir, sha, err := fake.Fetch("dep", Dep{Git: "ignored"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if dir != fixture || sha != "cafef00d" {
		t.Errorf("Fetch = (%q,%q), want (%q,cafef00d)", dir, sha, fixture)
	}
}

// fakeFetcher returns a fixed dir + sha, ignoring the dep. Network-free.
type fakeFetcher struct {
	dir string
	sha string
}

func (f fakeFetcher) Fetch(name string, dep Dep) (string, string, error) {
	return f.dir, f.sha, nil
}
