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

	rewritten := 0
	for _, path := range paths() {
		content, mode, err := readFile(root, path)
		if err != nil {
			return rewritten, err
		}
		updated := content
		for _, line := range doccounts.Lines() {
			if line.Path != path {
				continue
			}
			if updated, err = doccounts.Rewrite(updated, line, counts); err != nil {
				return rewritten, err
			}
		}
		if updated == content {
			continue
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(updated), mode); err != nil {
			return rewritten, err
		}
		rewritten++
		fmt.Fprintf(out, "doc-counts: rewrote %s\n", path)
	}
	return rewritten, nil
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
