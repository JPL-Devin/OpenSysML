// internal/core/deps/resolver.go
package deps

import (
	"os"
	"path/filepath"

	"github.com/Open-MBEE/Systemica/internal/core/libs"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Resolver turns a manifest's declared dependencies into local directories and
// loads their model files into a shared symbol index. Local path deps are used
// as-is; git deps are acquired through the Fetcher and pinned in the lock.
type Resolver struct {
	fetcher Fetcher
	lock    *Lock
	cache   *libs.Cache
	seen    map[string]bool // SHAs already loaded, for transitive dedup (Task 6)
}

// NewResolver builds a Resolver. cache may be nil (loads then skip persistence).
func NewResolver(fetcher Fetcher, lock *Lock, cache *libs.Cache) *Resolver {
	if lock == nil {
		lock = NewLock()
	}
	return &Resolver{
		fetcher: fetcher,
		lock:    lock,
		cache:   cache,
		seen:    map[string]bool{},
	}
}

// resolveDirs resolves each dependency to a local directory. Git deps are
// fetched and their resolved SHA recorded in the lock. Returned map is keyed by
// dependency name.
func (r *Resolver) resolveDirs(root string, m *Manifest) (map[string]string, error) {
	dirs := map[string]string{}
	if m == nil {
		return dirs, nil
	}
	for name, dep := range m.Dependencies {
		if dep.Git != "" {
			dir, sha, err := r.fetcher.Fetch(name, dep)
			if err != nil {
				return nil, err
			}
			dirs[name] = dir
			if sha != "" {
				r.lock.SHA[name] = sha
			}
			continue
		}
		// Local path dependency: resolve relative to the workspace root.
		p := dep.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		dirs[name] = p
	}
	return dirs, nil
}

// Resolve resolves the manifest's dependencies and library paths (relative to
// root) and loads their files into idx. Dependencies are walked transitively
// (a dependency's own sysml.toml is resolved recursively), deduped by resolved
// SHA (git) or absolute directory (local), which also breaks dependency cycles.
// Dependencies load after the caller's workspace files and before the bundled
// stdlib, matching the spec's workspace -> dependencies -> stdlib import order.
func (r *Resolver) Resolve(root string, m *Manifest, idx *symbols.Index) error {
	if m == nil {
		return nil
	}
	if err := r.loadTree(root, m, idx); err != nil {
		return err
	}
	// library-paths are leaf library directories (no nested-manifest recursion).
	for _, lp := range m.LibraryPaths {
		dir := lp
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		if err := loadDir(dir, idx, r.cache); err != nil {
			return err
		}
	}
	return nil
}

// loadTree resolves each dependency declared by m (relative to root) to a
// directory, loads that directory's files into idx exactly once (deduped via
// r.seen), then recurses into any sysml.toml the dependency itself carries.
func (r *Resolver) loadTree(root string, m *Manifest, idx *symbols.Index) error {
	for name, dep := range m.Dependencies {
		dir, sha, err := r.resolveDep(root, name, dep)
		if err != nil {
			return err
		}
		key := depKey(dir, sha)
		if r.seen[key] {
			continue // already loaded (dedup) / cycle back-edge (terminate)
		}
		r.seen[key] = true
		if err := loadDir(dir, idx, r.cache); err != nil {
			return err
		}
		// Recurse into the dependency's own manifest, if present. A missing
		// nested sysml.toml is not an error (leaf dependency).
		nested := filepath.Join(dir, "sysml.toml")
		data, err := os.ReadFile(nested)
		if err != nil {
			continue // no nested manifest (or unreadable): treat as leaf
		}
		sub, err := ParseManifest(data)
		if err != nil {
			return err
		}
		if err := r.loadTree(dir, sub, idx); err != nil {
			return err
		}
	}
	return nil
}

// resolveDep resolves a single dependency to a directory and (for git deps) a
// SHA, recording the SHA in the lock. Local path deps resolve relative to root.
func (r *Resolver) resolveDep(root, name string, dep Dep) (dir, sha string, err error) {
	if dep.Git != "" {
		dir, sha, err = r.fetcher.Fetch(name, dep)
		if err != nil {
			return "", "", err
		}
		if sha != "" {
			r.lock.SHA[name] = sha
		}
		return dir, sha, nil
	}
	p := dep.Path
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return p, "", nil
}

// depKey is the dedup/cycle key for a resolved dependency: the resolved commit
// SHA for git deps (immutable, content-identifying) or the absolute directory
// for local deps (which have no SHA).
func depKey(dir, sha string) string {
	if sha != "" {
		return "sha:" + sha
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return "dir:" + abs
}
