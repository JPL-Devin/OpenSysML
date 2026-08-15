package model

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

const examplesDir = "../../../examples"

// examplesKnownFailures records the shipped examples that are expected to
// report errors, keyed by path with the reason. README and QUICKSTART point new
// users at examples/, so an example that does not analyse cleanly is a bug
// unless it is listed here on purpose.
var examplesKnownFailures = map[string]string{}

// Every example the repository ships must analyse cleanly, so the CLI's
// -validate exit status stays meaningful for the files the docs point at.
func TestExamplesAnalyseCleanly(t *testing.T) {
	files := exampleFiles(t)
	if len(files) == 0 {
		t.Fatalf("no examples found under %s", examplesDir)
	}

	for _, rel := range files {
		t.Run(rel, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(examplesDir, rel))
			if err != nil {
				t.Fatalf("read %s: %v", rel, err)
			}

			ws := NewWorkspace()
			ws.Open(rel, content, 1)

			var errs []string
			for _, d := range ws.Diagnostics(rel) {
				if d.Severity == passes.SeverityError {
					errs = append(errs, d.Message)
				}
			}

			reason, known := examplesKnownFailures[rel]
			switch {
			case known && len(errs) == 0:
				t.Errorf("listed as a known failure (%s) but analyses cleanly; remove it from examplesKnownFailures", reason)
			case known:
				t.Logf("known failure (%s): %s", reason, strings.Join(errs, "; "))
			case len(errs) > 0:
				t.Errorf("%d error(s): %s", len(errs), strings.Join(errs, "; "))
			}
		})
	}
}

// exampleFiles are the example models, the vendored OMG training corpus aside:
// that corpus is a separate gate (TestTrainingExamplesSemanticErrors).
func exampleFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	err := filepath.Walk(examplesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(examplesDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if rel == "sysml-v2-training" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".sysml") || strings.HasSuffix(rel, ".kerml") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", examplesDir, err)
	}
	sort.Strings(files)
	return files
}
