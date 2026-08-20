// Command pilot-diff compares this implementation's diagnostics against the
// OMG SysML v2 Pilot Implementation over a corpus of models, and reports, per
// file, the diagnostics both agree on, the ones only we report (candidate false
// positives) and the ones only the pilot reports (candidate gaps).
//
// It is advisory: nothing in the build or the test suite depends on it, and it
// never touches internal/core/model/testdata/training_examples_expected.txt.
// Provision the reference validator with scripts/download-pilot-validator.sh,
// then run `go run ./cmd/pilot-diff`. See docs/project/pilot-differential.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// corpusRoot is one directory of models compared as a batch: every file in it is
// loaded before any diagnostic is read, on both sides, because corpus files
// import each other.
type corpusRoot struct {
	Name string
	Dir  string
	Lang language
	// Skip lists sub-paths of Dir (slash-separated, relative to Dir) that
	// belong to another root.
	Skip []string
}

type language uint8

const (
	languageSysML language = iota
	languageKerML
)

func (root corpusRoot) extension() string {
	if root.Lang == languageKerML {
		return ".kerml"
	}
	return ".sysml"
}

var defaultRoots = []corpusRoot{
	{Name: "training", Dir: "examples/sysml-v2-training"},
	{Name: "pilot-examples", Dir: "examples/pilot-corpora/sysml-examples"},
	{Name: "pilot-validation", Dir: "examples/pilot-corpora/sysml-validation"},
	// KerML is validated in one resource-set batch by the plain-Java bridge.
	{Name: "kerml-examples", Dir: "examples/pilot-corpora/kerml-examples", Lang: languageKerML},
	{Name: "testdata", Dir: "testdata"},
	{Name: "examples", Dir: "examples", Skip: []string{"sysml-v2-training", "pilot-corpora"}},
	// Hand-written models for behaviour classes the corpora do not cover, such
	// as redefining a feature inherited through an alias.
	{Name: "probes", Dir: "cmd/pilot-diff/testdata"},
}

func main() {
	repo := flag.String("repo", "", "repository root (default: the module root containing this command)")
	validator := flag.String("validator", "", "pilot validator executable (default: <repo>/build/pilot-validator/validate-sysml)")
	kermlValidator := flag.String("kerml-validator", "", "KerML pilot validator executable (default: <repo>/build/pilot-kerml-validator/validate-kerml)")
	out := flag.String("out", "", "output directory for the reports (default: <repo>/build/pilot-diff)")
	timeout := flag.Duration("timeout", 0, "per-batch timeout for the pilot validator (0: no limit)")
	flag.Parse()

	if err := run(*repo, *validator, *kermlValidator, *out, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "pilot-diff: %v\n", err)
		os.Exit(1)
	}
}

func run(repo, validator, kermlValidator, out string, timeout time.Duration) error {
	var err error
	if repo == "" {
		repo, err = moduleRoot()
		if err != nil {
			return err
		}
	}
	if validator == "" {
		validator = filepath.Join(repo, "build", "pilot-validator", "validate-sysml")
	}
	if kermlValidator == "" {
		kermlValidator = filepath.Join(repo, "build", "pilot-kerml-validator", "validate-kerml")
	}
	if out == "" {
		out = filepath.Join(repo, "build", "pilot-diff")
	}
	if _, err := os.Stat(validator); err != nil {
		return fmt.Errorf("pilot validator not found at %s: run ./scripts/download-pilot-validator.sh", validator)
	}

	// Recorded relative to the repository where possible: the JSON is committed
	// as a baseline, so it must not carry a machine-specific path.
	report := &Report{Validator: relativeTo(repo, validator)}
	if report.Pilot, err = pilotVersion(validator); err != nil {
		return err
	}

	for _, root := range defaultRoots {
		files, err := collectFiles(repo, root)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			fmt.Fprintf(os.Stderr, "skipping %s: no %s files (corpus not downloaded?)\n", root.Dir, root.extension())
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: %d file(s)\n", root.Name, len(files))

		pilot := validator
		if root.Lang == languageKerML {
			pilot = kermlValidator
			if _, err := os.Stat(pilot); err != nil {
				return fmt.Errorf("KerML pilot validator not found at %s: run ./scripts/download-pilot-kerml-validator.sh", pilot)
			}
		}
		ours, err := openSysMLDiagnostics(repo, root.Dir, files)
		if err != nil {
			return err
		}
		var theirs map[string][]diagnostic
		if root.Lang == languageKerML {
			theirs, err = kermlDiagnostics(pilot, repo, root.Dir, files, timeout)
		} else {
			theirs, err = pilotDiagnostics(pilot, repo, root.Dir, files, timeout)
		}
		if err != nil {
			return err
		}
		report.Roots = append(report.Roots, compareRoot(root.Name, root.Dir, files, ours, theirs))
	}

	report.summarize()
	// A mistyped -repo would otherwise look like a clean run.
	if report.Totals.Files == 0 {
		return fmt.Errorf("no .sysml files found under %s", repo)
	}
	return writeReports(out, report)
}

func relativeTo(repo, path string) string {
	rel, err := filepath.Rel(repo, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}

// moduleRoot walks up from the working directory to the directory holding go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above the working directory; pass -repo")
		}
		dir = parent
	}
}

// collectFiles returns the root's model files as sorted slash-separated paths
// relative to the root directory.
func collectFiles(repo string, root corpusRoot) ([]string, error) {
	dir := filepath.Join(repo, root.Dir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			for _, skip := range root.Skip {
				if rel == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) == root.extension() {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}
