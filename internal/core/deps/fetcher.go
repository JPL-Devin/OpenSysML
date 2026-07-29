// Package deps: fetcher.go
package deps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fetcher acquires a dependency's local directory, returning the directory
// and the resolved commit SHA (for git deps). Local-path deps are handled
// by the Resolver directly and never reach a Fetcher.
type Fetcher interface {
	Fetch(name string, dep Dep) (dir string, sha string, err error)
}

// gitFetcher shallow-clones git dependencies at a pinned rev into a cache
// directory, one checkout per (repo, rev). Read-only and pinned: an existing
// non-empty checkout is reused without re-cloning.
type gitFetcher struct {
	cacheDir string // e.g. $XDG_CACHE_HOME/sysml-ls/deps
}

// pinnedRev returns the explicit revision to check out: Rev, else Tag, else
// Branch. Empty means the default branch (git clone without --branch).
func (d Dep) pinnedRev() string {
	switch {
	case d.Rev != "":
		return d.Rev
	case d.Tag != "":
		return d.Tag
	case d.Branch != "":
		return d.Branch
	default:
		return ""
	}
}

// cacheDirFor derives the on-disk checkout path: <cacheDir>/<host>/<path>/<rev>.
// The git URL is split into host + path; rev falls back to "HEAD" when unpinned.
func (f *gitFetcher) cacheDirFor(name string, dep Dep) string {
	host, path := splitGitURL(dep.Git)
	rev := dep.pinnedRev()
	if rev == "" {
		rev = "HEAD"
	}
	segs := append([]string{f.cacheDir, host}, strings.Split(path, "/")...)
	segs = append(segs, rev)
	return filepath.Join(segs...)
}

// splitGitURL extracts a host and cleaned path from a git URL, dropping any
// scheme, user info, port, and trailing ".git". Best-effort; unparseable URLs
// yield host "" and the raw string as path.
func splitGitURL(url string) (host, path string) {
	s := url
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	if i := strings.Index(s, "/"); i >= 0 {
		host, path = s[:i], strings.Trim(s[i+1:], "/")
	} else {
		host, path = s, ""
	}
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	return host, path
}

func (f *gitFetcher) Fetch(name string, dep Dep) (string, string, error) {
	if dep.Git == "" {
		return "", "", fmt.Errorf("deps: %s: not a git dependency", name)
	}
	target := f.cacheDirFor(name, dep)
	rev := dep.pinnedRev()

	// Reuse an existing non-empty checkout (pinned deps are immutable).
	if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
		sha := rev
		if resolved, err := gitHeadSHA(target); err == nil && resolved != "" {
			sha = resolved
		}
		return target, sha, nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", "", err
	}
	args := []string{"clone", "--depth", "1"}
	if rev != "" {
		args = append(args, "--branch", rev)
	}
	args = append(args, dep.Git, target)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("deps: %s: git clone failed: %v: %s", name, err, out)
	}
	sha, err := gitHeadSHA(target)
	if err != nil {
		sha = rev
	}
	return target, sha, nil
}

// gitHeadSHA returns the resolved HEAD commit SHA of a checkout.
func gitHeadSHA(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
