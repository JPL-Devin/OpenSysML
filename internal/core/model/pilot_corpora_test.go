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

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

var updatePilotCorpora = flag.Bool("update-pilot-corpora", false,
	"rewrite testdata/pilot_corpora_expected.txt from the current results")

const (
	pilotCorporaParent   = "../../../examples/pilot-corpora"
	pilotCorporaExpected = "testdata/pilot_corpora_expected.txt"
	pilotCorporaSkipHint = "pilot corpora not downloaded (run ./scripts/download-pilot-corpora.sh)"

	// Set in CI so that an absent corpus fails the gate instead of skipping it.
	pilotCorporaRequiredEnv = "OPENSYSML_REQUIRE_PILOT_CORPORA"
)

// The pinned OMG pilot corpora, each fetched by scripts/download-pilot-corpora.sh.
var pilotCorporaRoots = []string{"kerml-examples", "sysml-examples", "sysml-validation"}

// skipWithoutPilotCorpora skips the calling test locally, but fails it when the
// corpora are declared mandatory, so the gate cannot pass by skipping. The skip
// is announced on stderr because `go test` hides skip reasons without -v, which
// makes a gate that never ran look like a gate that passed.
func skipWithoutPilotCorpora(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv(pilotCorporaRequiredEnv) != "" {
		t.Fatalf("%s=%s but %s: %s", pilotCorporaRequiredEnv, os.Getenv(pilotCorporaRequiredEnv), reason, pilotCorporaSkipHint)
	}
	fmt.Fprintf(os.Stderr, "\n!!! GATE NOT RUN: %s SKIPPED - %s.\n"+
		"!!! The OMG pilot corpora are absent, so this run proves nothing about them.\n"+
		"!!! Fetch them with ./scripts/download-pilot-corpora.sh and re-run\n"+
		"!!!   go test -count=1 ./internal/core/model -run TestPilotCorpora\n"+
		"!!! CI sets %s=1, where an absent corpus fails instead of skipping.\n\n",
		t.Name(), reason, pilotCorporaRequiredEnv)
	t.Skip(pilotCorporaSkipHint)
}

// The three pinned OMG pilot corpora are a regression gate on our own verdicts:
// every file's diagnostic count is recorded in
// testdata/pilot_corpora_expected.txt, so a file that starts reporting, stops
// reporting, or changes its number of diagnostics fails this test. It says
// nothing about whether those diagnostics are right — the comparison against the
// reference implementation is cmd/pilot-diff, which needs the pinned Java
// validators and stays out of CI. The corpora are not vendored (see
// scripts/download-pilot-corpora.sh), so the test skips when they are absent —
// unless OPENSYSML_REQUIRE_PILOT_CORPORA is set, as CI does.
func TestPilotCorporaDiagnostics(t *testing.T) {
	roots := pilotCorporaFiles(t)

	// Measure the implementation rather than the developer's machine: an empty
	// semantic cache makes the run index the standard library by parsing it,
	// which is what a fresh checkout, the LSP on a new machine, and CI all do.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got := pilotCorporaDiagnosticCounts(t, roots)

	if *updatePilotCorpora {
		writePilotCorporaExpected(t, roots, got)
		return
	}

	totals, want := readPilotCorporaExpected(t)
	for _, root := range pilotCorporaRoots {
		if totals[root] != len(roots[root]) {
			t.Errorf("%s holds %d model file(s), expectations were recorded against %d; "+
				"re-download the pinned corpora or regenerate with -update-pilot-corpora",
				root, len(roots[root]), totals[root])
		}
	}

	for _, path := range sortedKeys(want) {
		switch gotCount, ok := got[path]; {
		case !ok:
			t.Errorf("%s: expected %d diagnostic(s) but the file is now clean; "+
				"regenerate with -update-pilot-corpora", path, want[path])
		case gotCount != want[path]:
			direction := "more"
			if gotCount < want[path] {
				direction = "fewer"
			}
			t.Errorf("%s: %d diagnostic(s), expected %d (%s than recorded); "+
				"adjudicate the change, then regenerate with -update-pilot-corpora",
				path, gotCount, want[path], direction)
		}
	}
	for _, path := range sortedKeys(got) {
		if _, ok := want[path]; !ok {
			t.Errorf("%s: %d new diagnostic(s), previously clean; "+
				"adjudicate the change, then regenerate with -update-pilot-corpora",
				path, got[path])
		}
	}

	for _, root := range pilotCorporaRoots {
		reporting := 0
		for _, rel := range roots[root] {
			if got[root+"/"+rel] > 0 {
				reporting++
			}
		}
		t.Logf("%s: %d/%d pilot corpus files clean", root, len(roots[root])-reporting, len(roots[root]))
	}
}

// The persistent library cache is a performance optimisation: restoring a
// reduced record instead of parsing the library must not change a single
// diagnostic. TestTrainingExamplesCacheStateIndependent pins that for SysML;
// this is the KerML half, which the training corpus does not contain. Both runs
// share one cache directory, so the first populates it and the second reads it back.
func TestPilotCorporaCacheStateIndependent(t *testing.T) {
	roots := pilotCorporaFiles(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	cold := pilotCorporaDiagnosticCounts(t, roots)
	warm := pilotCorporaDiagnosticCounts(t, roots)

	for _, path := range sortedKeys(cold) {
		if warm[path] != cold[path] {
			t.Errorf("%s: %d diagnostic(s) on an empty cache, %d on a populated one",
				path, cold[path], warm[path])
		}
	}
	for _, path := range sortedKeys(warm) {
		if _, ok := cold[path]; !ok {
			t.Errorf("%s: clean on an empty cache, %d diagnostic(s) on a populated one",
				path, warm[path])
		}
	}
}

// pilotCorporaDiagnosticCounts opens each root in one workspace per language and
// returns the number of diagnostics per "<root>/<path>", logging the messages.
func pilotCorporaDiagnosticCounts(t *testing.T, roots map[string][]string) map[string]int {
	t.Helper()

	got := make(map[string]int)
	for _, root := range pilotCorporaRoots {
		dir := filepath.Join(pilotCorporaParent, root)
		for _, batch := range pilotCorporaBatches(roots[root]) {
			for key, count := range pilotCorpusBatchCounts(t, root, dir, batch) {
				got[key] = count
			}
		}
	}
	return got
}

// pilotCorpusBatchCounts loads one language batch of a root and counts its
// diagnostics. Every file is opened before any diagnostic is read, because the
// corpora import across files: diagnosing a file while later ones are still
// unopened would measure the alphabetical order of the corpus rather than the
// implementation.
func pilotCorpusBatchCounts(t *testing.T, root, dir string, files []string) map[string]int {
	t.Helper()

	got := make(map[string]int, len(files))
	ws := NewWorkspace()
	current := ""
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic while loading %s/%s: %v", root, current, r)
		}
	}()
	for _, rel := range files {
		current = rel
		content, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatalf("read %s/%s: %v", root, rel, err)
		}
		ws.Open(rel, content, 1)
	}

	for _, rel := range files {
		current = rel

		var messages []string
		for _, d := range ws.Diagnostics(rel) {
			messages = append(messages, d.Severity.String()+": "+d.Message)
		}
		if len(messages) > 0 {
			got[root+"/"+rel] = len(messages)
			t.Logf("%s/%s: %s", root, rel, strings.Join(messages, "; "))
		}
	}
	return got
}

// pilotCorporaBatches splits a root's files into one batch per language, SysML
// first, mirroring cmd/pilot-diff: a KerML file and a SysML file do not share a
// resource set there, so they must not share a workspace here either.
func pilotCorporaBatches(files []string) [][]string {
	var sysml, kerml []string
	for _, rel := range files {
		if source.KindOf(rel) == source.KindKerML {
			kerml = append(kerml, rel)
			continue
		}
		sysml = append(sysml, rel)
	}
	var batches [][]string
	for _, batch := range [][]string{sysml, kerml} {
		if len(batch) > 0 {
			batches = append(batches, batch)
		}
	}
	return batches
}

// pilotCorporaFiles returns each root's model files as sorted slash-separated
// paths relative to that root.
func pilotCorporaFiles(t *testing.T) map[string][]string {
	t.Helper()

	roots := make(map[string][]string, len(pilotCorporaRoots))
	for _, root := range pilotCorporaRoots {
		dir := filepath.Join(pilotCorporaParent, root)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			skipWithoutPilotCorpora(t, dir+" is missing")
		}

		var files []string
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || source.KindOf(path) == source.KindUnknown {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
		sort.Strings(files)
		// An empty directory is a broken download or a bad cache restore, not a corpus.
		if len(files) == 0 {
			skipWithoutPilotCorpora(t, dir+" holds no model files")
		}
		roots[root] = files
	}
	return roots
}

// readPilotCorporaExpected returns the recorded size of each root and the
// expected diagnostic count per "<root>/<path>". Lines are "<count>\t<path>";
// blank and #-prefixed lines are comments, and a root's size is recorded as
// "# files: <root> <n>".
func readPilotCorporaExpected(t *testing.T) (map[string]int, map[string]int) {
	t.Helper()

	content, err := os.ReadFile(pilotCorporaExpected)
	if err != nil {
		t.Fatalf("read %s: %v", pilotCorporaExpected, err)
	}

	totals := make(map[string]int, len(pilotCorporaRoots))
	want := make(map[string]int)
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(line, "# files: "); ok {
			root, count, found := strings.Cut(rest, " ")
			if !found {
				t.Fatalf("%s:%d: want \"# files: <root> <n>\", got %q", pilotCorporaExpected, i+1, line)
			}
			totals[root], err = strconv.Atoi(count)
			if err != nil {
				t.Fatalf("%s:%d: bad file count: %v", pilotCorporaExpected, i+1, err)
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		count, path, found := strings.Cut(line, "\t")
		if !found {
			t.Fatalf("%s:%d: want \"<count>\\t<path>\", got %q", pilotCorporaExpected, i+1, line)
		}
		n, err := strconv.Atoi(count)
		if err != nil {
			t.Fatalf("%s:%d: bad count: %v", pilotCorporaExpected, i+1, err)
		}
		want[path] = n
	}
	return totals, want
}

func writePilotCorporaExpected(t *testing.T, roots map[string][]string, got map[string]int) {
	t.Helper()

	var b strings.Builder
	b.WriteString("# Files in the pinned OMG pilot corpora that report diagnostics of any\n")
	b.WriteString("# severity, as \"<diagnostic count>\\t<root>/<path>\". These are our verdicts\n")
	b.WriteString("# alone, not a comparison against the reference implementation; see\n")
	b.WriteString("# docs/project/pilot-corpora.md. Regenerate with:\n")
	b.WriteString("#   go test ./internal/core/model -run TestPilotCorporaDiagnostics -update-pilot-corpora\n")
	for _, root := range pilotCorporaRoots {
		fmt.Fprintf(&b, "# files: %s %d\n", root, len(roots[root]))
	}
	for _, path := range sortedKeys(got) {
		fmt.Fprintf(&b, "%d\t%s\n", got[path], path)
	}

	if err := os.WriteFile(pilotCorporaExpected, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", pilotCorporaExpected, err)
	}
	t.Logf("wrote %s: %d file(s) reporting diagnostics", pilotCorporaExpected, len(got))
}
