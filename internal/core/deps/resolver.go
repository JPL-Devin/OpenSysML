// internal/core/deps/resolver.go
package deps

import (
	"path/filepath"

	"github.com/Open-MBEE/Systemica/internal/core/libs"
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
