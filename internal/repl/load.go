package repl

import (
	"errors"
	"fmt"
	"os"

	"github.com/Open-MBEE/Systemica/internal/core/project"
)

// LoadPaths loads model files into the session. Each path names a file, a
// directory to walk for .sysml/.kerml files, or a glob pattern. Every file is
// accepted before the buffer is analyzed, so the order files are loaded in does
// not affect name resolution: a file may reference a declaration another file
// loaded after it makes. Diagnostics name the file they belong to and count
// lines from its start.
func (s *Session) LoadPaths(paths []string) ([]string, error) {
	files, err := ExpandPaths(paths)
	if err != nil {
		return nil, err
	}
	srcs := make([]SourceFile, 0, len(files))
	for _, file := range files {
		// #nosec G304 -- the file is one the user named, or one found under a
		// directory or pattern they named.
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", file, err)
		}
		srcs = append(srcs, SourceFile{Name: file, Text: string(data)})
	}
	var out []string
	if len(files) > 1 {
		out = append(out, fmt.Sprintf("loaded %d files:", len(files)))
		for _, file := range files {
			out = append(out, "  "+file)
		}
	}
	return append(out, renderResult(s.submitFiles(srcs), s.verbosity)...), nil
}

// ExpandPaths turns the paths a caller was given — files, directories to walk
// for .sysml/.kerml files, or glob patterns — into the model files to load, in a
// deterministic order and without duplicates.
func ExpandPaths(paths []string) ([]string, error) {
	files, err := project.Expand(expandHomes(paths))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("no model files to load")
	}
	return files, nil
}

// expandHomes expands a leading ~ in every path.
func expandHomes(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, expandHome(p))
	}
	return out
}
