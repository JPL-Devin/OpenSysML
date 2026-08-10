package model

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

var updateTraining = flag.Bool("update-training", false,
	"rewrite testdata/training_examples_expected.txt from the current results")

const (
	trainingDir      = "../../../examples/sysml-v2-training"
	trainingExpected = "testdata/training_examples_expected.txt"
	trainingSkipHint = "training examples not downloaded (run ./scripts/download-training-examples.sh)"
)

// The OMG training corpus is a regression gate, not a report: every file that
// still reports semantic errors is recorded with its error count in
// testdata/training_examples_expected.txt, so a file that starts failing, stops
// failing, or changes its number of errors fails this test. The corpus itself is
// not vendored (see scripts/download-training-examples.sh), so the test skips
// when it is absent.
func TestTrainingExamplesSemanticErrors(t *testing.T) {
	files := trainingFiles(t)

	// Measure the implementation rather than the developer's machine: an empty
	// semantic cache makes the run index the standard library by parsing it,
	// which is what a fresh checkout, the LSP on a new machine, and CI all do.
	// TestTrainingExamplesCacheStateIndependent pins that the restored-from-
	// cache run agrees with it.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got := trainingErrorCounts(t, files)

	if *updateTraining {
		writeTrainingExpected(t, len(files), got)
		return
	}

	total, want := readTrainingExpected(t)
	if total != len(files) {
		t.Errorf("corpus has %d .sysml files, expectations were recorded against %d; "+
			"re-download the pinned corpus or regenerate with -update-training",
			len(files), total)
	}

	for _, path := range sortedKeys(want) {
		switch gotCount, ok := got[path]; {
		case !ok:
			t.Errorf("%s: expected %d error(s) but the file is now clean; "+
				"remove it from %s", path, want[path], trainingExpected)
		case gotCount != want[path]:
			t.Errorf("%s: %d error(s), expected %d", path, gotCount, want[path])
		}
	}
	for _, path := range sortedKeys(got) {
		if _, ok := want[path]; !ok {
			t.Errorf("%s: %d new error(s), previously clean", path, got[path])
		}
	}

	t.Logf("%d/%d training files clean", len(files)-len(got), len(files))
}

// trainingErrorCounts opens the whole corpus in one workspace and returns the
// number of error diagnostics per file, logging each file's messages.
func trainingErrorCounts(t *testing.T, files []string) map[string]int {
	t.Helper()

	got := make(map[string]int, len(files))
	ws := NewWorkspace()
	current := ""
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic while loading %s: %v", current, r)
		}
	}()
	// Open every file before reading any diagnostics: the corpus imports across
	// files (`private import 'Requirement Usages'::*`), so diagnosing a file
	// while later ones are still unopened would measure the alphabetical order
	// of the corpus rather than the implementation.
	for _, path := range files {
		current = path
		content, err := os.ReadFile(filepath.Join(trainingDir, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		ws.Open(path, content, 1)
	}

	for _, path := range files {
		current = path

		var errs []string
		for _, d := range ws.Diagnostics(path) {
			if d.Severity == passes.SeverityError {
				errs = append(errs, d.Message)
			}
		}
		if len(errs) > 0 {
			got[path] = len(errs)
			t.Logf("%s: %s", path, strings.Join(errs, "; "))
		}
	}
	return got
}

// The persistent library cache is a performance optimisation: restoring a
// reduced record instead of parsing the library must not change a single
// diagnostic. Both runs share one cache directory, so the first populates it
// and the second reads it back.
func TestTrainingExamplesCacheStateIndependent(t *testing.T) {
	files := trainingFiles(t)
	if testing.Short() {
		t.Skip("indexes the whole corpus twice")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	cold := trainingErrorCounts(t, files)
	warm := trainingErrorCounts(t, files)

	for _, path := range sortedKeys(cold) {
		if warm[path] != cold[path] {
			t.Errorf("%s: %d error(s) on an empty cache, %d on a populated one",
				path, cold[path], warm[path])
		}
	}
	for _, path := range sortedKeys(warm) {
		if _, ok := cold[path]; !ok {
			t.Errorf("%s: clean on an empty cache, %d error(s) on a populated one",
				path, warm[path])
		}
	}
}

// TestRequirementDefinitionsFile pins one training file that exercises
// requirement definitions end to end, because the corpus gate above only counts
// errors per file.
func TestRequirementDefinitionsFile(t *testing.T) {
	const name = "32. Requirements/Requirement Definitions.sysml"

	content, err := os.ReadFile(filepath.Join(trainingDir, name))
	if os.IsNotExist(err) {
		t.Skip(trainingSkipHint)
	}
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	ws := NewWorkspace()
	ws.Open(name, content, 1)

	for _, d := range ws.Diagnostics(name) {
		if d.Severity == passes.SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func trainingFiles(t *testing.T) []string {
	t.Helper()

	if _, err := os.Stat(trainingDir); os.IsNotExist(err) {
		t.Skip(trainingSkipHint)
	}

	var files []string
	err := filepath.Walk(trainingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".sysml") {
			return nil
		}
		rel, err := filepath.Rel(trainingDir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", trainingDir, err)
	}
	sort.Strings(files)
	return files
}

// readTrainingExpected returns the recorded corpus size and the expected error
// count per file. Lines are "<count>\t<path>"; blank and #-prefixed lines are
// comments, and the corpus size is recorded as "# files: <n>".
func readTrainingExpected(t *testing.T) (int, map[string]int) {
	t.Helper()

	content, err := os.ReadFile(trainingExpected)
	if err != nil {
		t.Fatalf("read %s: %v", trainingExpected, err)
	}

	total := 0
	want := make(map[string]int)
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "# files: ") {
			total, err = strconv.Atoi(strings.TrimPrefix(line, "# files: "))
			if err != nil {
				t.Fatalf("%s:%d: bad file count: %v", trainingExpected, i+1, err)
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		count, path, found := strings.Cut(line, "\t")
		if !found {
			t.Fatalf("%s:%d: want \"<count>\\t<path>\", got %q", trainingExpected, i+1, line)
		}
		n, err := strconv.Atoi(count)
		if err != nil {
			t.Fatalf("%s:%d: bad count: %v", trainingExpected, i+1, err)
		}
		want[path] = n
	}
	return total, want
}

func writeTrainingExpected(t *testing.T, total int, got map[string]int) {
	t.Helper()

	var b strings.Builder
	b.WriteString("# Files in the pinned OMG training corpus that still report semantic errors,\n")
	b.WriteString("# as \"<error count>\\t<path>\". See docs/TRAINING_EXAMPLES.md for why each one\n")
	b.WriteString("# fails; regenerate with:\n")
	b.WriteString("#   go test ./internal/core/model -run TestTrainingExamplesSemanticErrors -update-training\n")
	fmt.Fprintf(&b, "# files: %d\n", total)
	for _, path := range sortedKeys(got) {
		fmt.Fprintf(&b, "%d\t%s\n", got[path], path)
	}

	if err := os.WriteFile(trainingExpected, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", trainingExpected, err)
	}
	t.Logf("wrote %s: %d/%d files clean", trainingExpected, total-len(got), total)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
