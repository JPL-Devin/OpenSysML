// Command pilot-diff compares this implementation's diagnostics against the
// OMG SysML v2 Pilot Implementation over a corpus of models, and reports, per
// file, the diagnostics both agree on, the ones only we report (candidate false
// positives) and the ones only the pilot reports (candidate gaps).
//
// It is advisory: nothing in the build or the test suite depends on it, and it
// never touches internal/core/model/testdata/training_examples_expected.txt.
// Provision the reference validator with scripts/download-pilot-sysml-validator.sh,
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

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// corpusRoot is one directory of models. Each language in it is compared as its
// own single batch: every file of that language is loaded into one resource set
// before any diagnostic is read, on both sides, because corpus files import
// each other.
type corpusRoot struct {
	Name string
	Dir  string
	// Skip lists sub-paths of Dir (slash-separated, relative to Dir) that
	// belong to another root.
	Skip []string
}

// languageBatch is one root's files in a single language, in comparison order.
type languageBatch struct {
	Kind  source.Kind
	Files []string
}

var defaultRoots = []corpusRoot{
	{Name: "training", Dir: "examples/sysml-v2-training"},
	{Name: "pilot-examples", Dir: "examples/pilot-corpora/sysml-examples"},
	{Name: "pilot-validation", Dir: "examples/pilot-corpora/sysml-validation"},
	{Name: "kerml-examples", Dir: "examples/pilot-corpora/kerml-examples"},
	{Name: "testdata", Dir: "testdata"},
	{Name: "examples", Dir: "examples", Skip: []string{"sysml-v2-training", "pilot-corpora"}},
	// Hand-written models for behaviour classes the corpora do not cover, such
	// as redefining a feature inherited through an alias.
	{Name: "probes", Dir: "cmd/pilot-diff/testdata"},
}

func main() {
	repo := flag.String("repo", "", "repository root (default: the module root containing this command)")
	validator := flag.String("validator", "", "pilot SysML validator executable (default: <repo>/build/pilot-sysml-validator/validate-sysml-batch)")
	kermlValidator := flag.String("kerml-validator", "", "KerML pilot validator executable (default: <repo>/build/pilot-kerml-validator/validate-kerml)")
	syside := flag.String("syside", "", "optional Sensmetry SysIDE launcher for a third column (default: <repo>/build/syside/validate-syside if present)")
	out := flag.String("out", "", "output directory for the reports (default: <repo>/build/pilot-diff)")
	timeout := flag.Duration("timeout", 0, "per-batch timeout for the pilot validator (0: no limit)")
	flag.Parse()

	if err := run(*repo, *validator, *kermlValidator, *syside, *out, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "pilot-diff: %v\n", err)
		os.Exit(1)
	}
}

func run(repo, validator, kermlValidator, syside, out string, timeout time.Duration) error {
	var err error
	if repo == "" {
		repo, err = moduleRoot()
		if err != nil {
			return err
		}
	}
	if validator == "" {
		validator = filepath.Join(repo, "build", "pilot-sysml-validator", "validate-sysml-batch")
	}
	if kermlValidator == "" {
		kermlValidator = filepath.Join(repo, "build", "pilot-kerml-validator", "validate-kerml")
	}
	if out == "" {
		out = filepath.Join(repo, "build", "pilot-diff")
	}
	if _, err := os.Stat(validator); err != nil {
		return fmt.Errorf("pilot validator not found at %s: run ./scripts/download-pilot-sysml-validator.sh", validator)
	}

	// Named explicitly: fail loudly. Defaulted: the third column is optional,
	// and its absence must leave the two-way report byte-identical.
	requested := syside != ""
	if !requested {
		syside = filepath.Join(repo, "build", "syside", "validate-syside")
	}
	if _, err := os.Stat(syside); err != nil {
		if requested {
			return fmt.Errorf("SysIDE launcher not found at %s: run ./scripts/download-syside.sh", syside)
		}
		fmt.Fprintf(os.Stderr, "comparing against the pilot only; run ./scripts/download-syside.sh for a third column\n")
		syside = ""
	}

	// Recorded relative to the repository where possible: the JSON is committed
	// as a baseline, so it must not carry a machine-specific path.
	report := &Report{Validator: relativeTo(repo, validator)}
	if report.Pilot, err = pilotVersion(validator); err != nil {
		return err
	}
	if syside != "" {
		version, library, err := sysideRelease(syside)
		if err != nil {
			return err
		}
		report.Syside = &SysideInfo{
			Validator: relativeTo(repo, syside),
			Version:   version,
			Library:   library,
			Pilot:     report.Pilot,
			Scope:     sysideScope,
		}
	}

	for _, root := range defaultRoots {
		files, err := collectFiles(repo, root)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			fmt.Fprintf(os.Stderr, "skipping %s: no .sysml or .kerml files (corpus not downloaded?)\n", root.Dir)
			continue
		}

		ours := make(map[string][]diagnostic, len(files))
		theirs := make(map[string][]diagnostic, len(files))
		for _, batch := range batchByLanguage(files) {
			fmt.Fprintf(os.Stderr, "%s: %d %s file(s)\n", root.Name, len(batch.Files), batch.Kind)

			pilot := validator
			if batch.Kind == source.KindKerML {
				pilot = kermlValidator
				if _, err := os.Stat(pilot); err != nil {
					return fmt.Errorf("KerML pilot validator not found at %s: run ./scripts/download-pilot-kerml-validator.sh", pilot)
				}
			}
			batchOurs, err := openSysMLDiagnostics(repo, root.Dir, batch.Files)
			if err != nil {
				return err
			}
			batchTheirs, err := pilotDiagnostics(pilot, repo, root.Dir, batch.Files, timeout)
			if err != nil {
				return err
			}
			for rel, diagnostics := range batchOurs {
				ours[rel] = diagnostics
			}
			for rel, diagnostics := range batchTheirs {
				theirs[rel] = diagnostics
			}
		}
		rootReport := compareRoot(root.Name, root.Dir, files, ours, theirs)
		if syside != "" {
			fmt.Fprintf(os.Stderr, "%s: %d file(s) through syside\n", root.Name, len(files))
			third, err := sysideDiagnostics(syside, repo, root.Dir, files, timeout)
			if err != nil {
				return err
			}
			attachSyside(&rootReport, files, ours, theirs, third)
		}
		report.Roots = append(report.Roots, rootReport)
	}

	report.summarize()
	// A mistyped -repo would otherwise look like a clean run.
	if report.Totals.Files == 0 {
		return fmt.Errorf("no model files found under %s", repo)
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

// batchByLanguage splits a root's files into one batch per language, SysML
// first. Each language is compared against its own reference validator, and
// batching per language rather than per file keeps the reference's cross-file
// reference resolution intact within a batch.
func batchByLanguage(files []string) []languageBatch {
	batches := []languageBatch{{Kind: source.KindSysML}, {Kind: source.KindKerML}}
	for _, rel := range files {
		for i := range batches {
			if batches[i].Kind == source.KindOf(rel) {
				batches[i].Files = append(batches[i].Files, rel)
			}
		}
	}
	out := make([]languageBatch, 0, len(batches))
	for _, batch := range batches {
		if len(batch.Files) > 0 {
			out = append(out, batch)
		}
	}
	return out
}

// collectFiles returns the root's model files, in either language, as sorted
// slash-separated paths relative to the root directory.
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
		if source.KindOf(path) != source.KindUnknown {
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
