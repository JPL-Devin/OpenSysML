// Package libs loads SysML/KerML standard-library files (bundled via embed.FS
// or overridden on disk) and maintains a persistent cache of their indexed
// symbols. See spec section 10.
package libs

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed stdlib/*.kerml
var stdlibFS embed.FS

// Source yields standard-library file contents by logical file name (base
// name, e.g. "ScalarValues.kerml").
type Source interface {
	// List returns the logical names of available library files, sorted.
	List() []string
	// Read returns the bytes of the named library file, or an error.
	Read(name string) ([]byte, error)
}

// DefaultSource returns a dirSource rooted at SYSML_LIBRARY_PATH when that
// environment variable is set and non-empty, otherwise the embedded source.
func DefaultSource() Source {
	if dir := os.Getenv("SYSML_LIBRARY_PATH"); dir != "" {
		return &dirSource{dir: dir}
	}
	return &embedSource{}
}

// NewDirSource returns a Source that reads .kerml/.sysml files from dir.
func NewDirSource(dir string) Source {
	return &dirSource{dir: dir}
}

type embedSource struct{}

func (s *embedSource) List() []string {
	entries, err := stdlibFS.ReadDir("stdlib")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func (s *embedSource) Read(name string) ([]byte, error) {
	return stdlibFS.ReadFile("stdlib/" + name)
}

type dirSource struct{ dir string }

func (s *dirSource) List() []string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".kerml") || strings.HasSuffix(n, ".sysml") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func (s *dirSource) Read(name string) ([]byte, error) {
	if name != filepath.Base(name) {
		return nil, fmt.Errorf("libs: invalid library file name %q", name)
	}
	return os.ReadFile(filepath.Join(s.dir, name))
}
