// Command doc-counts rewrites the documentation lines that are a function of the
// compliance map's status markers, so no contributor types them. It reads the
// census through internal/doccounts, which the guard in cmd/pilot-diff reads too,
// and rewrites nothing else in the files it touches. Run it with `make docs-counts`.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/doccounts"
)

func main() {
	root := flag.String("root", ".", "repository root the documentation paths are relative to")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "doc-counts: unexpected argument %q\n", flag.Arg(0))
		os.Exit(2)
	}
	rewritten, err := run(*root, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doc-counts: %v\n", err)
		os.Exit(1)
	}
	if rewritten == 0 {
		fmt.Fprintln(os.Stdout, "doc-counts: already current")
	}
}

// run restates every derived line from the census and reports how many files it
// changed. A file already stating the census is left untouched, which is what
// makes a second run a no-op.
func run(root string, out io.Writer) (int, error) {
	compliance, _, err := readFile(root, doccounts.SpecCompliancePath)
	if err != nil {
		return 0, err
	}
	counts := doccounts.CountRules(compliance)
	if counts.Total == 0 {
		return 0, fmt.Errorf("%s states no rule rows, so there is no census to write", doccounts.SpecCompliancePath)
	}

	// Every file is rewritten in memory, and checked writable, before any is
	// written, so a failure on a later file cannot leave an earlier one restated
	// against a census the rest of the tree does not state.
	type rewrite struct {
		path    string
		content string
		mode    fs.FileMode
	}
	var pending []rewrite
	for _, path := range paths() {
		content, mode, err := readFile(root, path)
		if err != nil {
			return 0, err
		}
		updated := content
		for _, line := range doccounts.Lines() {
			if line.Path != path {
				continue
			}
			if updated, err = doccounts.Rewrite(updated, line, counts); err != nil {
				return 0, err
			}
		}
		if updated == content {
			continue
		}
		if err := checkWritable(root, path); err != nil {
			return 0, err
		}
		pending = append(pending, rewrite{path: path, content: updated, mode: mode})
	}

	for _, file := range pending {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(file.path)), []byte(file.content), file.mode); err != nil {
			return 0, err
		}
		fmt.Fprintf(out, "doc-counts: rewrote %s\n", file.path)
	}
	return len(pending), nil
}

// checkWritable reports whether a file can be opened for writing, so an
// unwritable file is refused before any file is rewritten.
func checkWritable(root, path string) error {
	file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(path)), os.O_WRONLY, 0) // #nosec G304 -- a documentation path this command declares
	if err != nil {
		return err
	}
	return file.Close()
}

// paths lists the files carrying a derived line, in the order the lines declare
// them and without repeating a file that carries more than one.
func paths() []string {
	var ordered []string
	seen := map[string]bool{}
	for _, line := range doccounts.Lines() {
		if seen[line.Path] {
			continue
		}
		seen[line.Path] = true
		ordered = append(ordered, line.Path)
	}
	return ordered
}

func readFile(root, path string) (string, fs.FileMode, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Stat(full)
	if err != nil {
		return "", 0, err
	}
	content, err := os.ReadFile(full) // #nosec G304 -- a documentation path this command declares
	if err != nil {
		return "", 0, err
	}
	return string(content), info.Mode().Perm(), nil
}
