package export_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

var updateCorpusRoundTrip = flag.Bool("update-corpus-round-trip", false,
	"rewrite testdata/corpus_round_trip_expected.txt from the current results")

const corpusRoundTripExpected = "testdata/corpus_round_trip_expected.txt"

// Byte-identical to the committed file's header, so regenerating without a
// movement rewrites it unchanged.
const corpusRoundTripHeader = "# Example models this mapping converts whose second conversion,\n" +
	"# notation -> Turtle -> notation -> Turtle, does not give back the first\n" +
	"# graph byte for byte, as \"<outcome>\\t<root>/<path>\":\n" +
	"#   unwritable     the graph does not write back to notation\n" +
	"#   unreadable     the written notation does not convert again\n" +
	"#   graph-differs  the second graph states something else\n" +
	"# Models the mapping refuses are not recorded. Regenerate with:\n" +
	"#   go test ./internal/core/export -run TestCorpusSecondConversionIsByteStable -update-corpus-round-trip\n"

// A corpusRoot is one tree of example models. The committed examples are always
// present; the downloaded corpora skip when absent unless requireEnv is set, as
// it is in CI.
type corpusRoot struct {
	name       string
	dir        string
	within     []string // subdirectories that are roots of their own
	requireEnv string
	fetch      string
}

var corpusRoots = []corpusRoot{
	{
		name:   "examples",
		dir:    "../../../examples",
		within: []string{"sysml-v2-training", "pilot-corpora"},
	},
	{
		name:       "sysml-v2-training",
		dir:        "../../../examples/sysml-v2-training",
		requireEnv: "OPENSYSML_REQUIRE_TRAINING_CORPUS",
		fetch:      "./scripts/download-training-examples.sh",
	},
	{
		name:       "pilot-corpora",
		dir:        "../../../examples/pilot-corpora",
		requireEnv: "OPENSYSML_REQUIRE_PILOT_CORPORA",
		fetch:      "./scripts/download-pilot-corpora.sh",
	},
}

// Every example this mapping converts must convert to the same graph twice:
// the Turtle it writes, written back to notation and converted again, must be
// byte-identical, stored sysx:sourceText literals included. That is what makes
// the stored text trustworthy layout rather than something each hop rewrites.
// The files that do not yet get that far are a per-file ratchet in
// testdata/corpus_round_trip_expected.txt, so a file that starts or stops being
// byte-stable, or changes why it is not, fails this test until adjudicated.
func TestCorpusSecondConversionIsByteStable(t *testing.T) {
	want, totals := readCorpusRoundTripExpected(t)

	got := make(map[string]string)
	counts := make(map[string]int)
	converted := 0
	for _, root := range corpusRoots {
		files, present := root.files(t)
		if !present {
			continue
		}
		counts[root.name] = len(files)
		for _, rel := range files {
			key := root.name + "/" + rel
			src, err := os.ReadFile(filepath.Join(root.dir, rel))
			if err != nil {
				t.Fatalf("read %s: %v", key, err)
			}
			outcome, refused := secondConversion(rel, src)
			if refused {
				continue
			}
			converted++
			if outcome != "" {
				got[key] = outcome
				t.Logf("%s: %s", key, outcome)
			}
		}
	}
	t.Logf("%d of %d converted example models are byte-stable", converted-len(got), converted)

	if *updateCorpusRoundTrip {
		for _, root := range corpusRoots {
			if _, ok := counts[root.name]; !ok {
				t.Fatalf("%s is absent; fetch it with %s before regenerating", root.name, root.fetch)
			}
		}
		writeCorpusRoundTripExpected(t, counts, got)
		return
	}

	for root, n := range counts {
		if totals[root] != n {
			t.Errorf("%s holds %d files, expectations recorded against %d; re-fetch it or regenerate with -update-corpus-round-trip",
				root, n, totals[root])
		}
	}
	for key, outcome := range got {
		switch want[key] {
		case outcome:
		case "":
			t.Errorf("%s: %s, expected byte-stable; adjudicate the regression, then regenerate with -update-corpus-round-trip", key, outcome)
		default:
			t.Errorf("%s: %s, expected %s; adjudicate the change, then regenerate with -update-corpus-round-trip", key, outcome, want[key])
		}
	}
	for key, outcome := range want {
		root, _, _ := strings.Cut(key, "/")
		if _, present := counts[root]; present && got[key] == "" {
			t.Errorf("%s: byte-stable, expected %s; regenerate with -update-corpus-round-trip to record the gain", key, outcome)
		}
	}
}

// secondConversion converts one model notation → Turtle → notation → Turtle. It
// reports refused when the mapping does not convert the model at all, and
// otherwise why the second graph is not the first, or "" when it is.
func secondConversion(name string, src []byte) (outcome string, refused bool) {
	first, err := export.Convert(name, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		return "", true
	}
	back, err := export.Convert(name+".ttl", first, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		return "unwritable", false
	}
	second, err := export.Convert(name, back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		return "unreadable", false
	}
	if string(first) != string(second) {
		return "graph-differs", false
	}
	return "", false
}

// files lists a root's models relative to its directory, sorted, and whether
// the root is present. An absent root is announced and skipped, or fails when
// its environment variable requires it.
func (r corpusRoot) files(t *testing.T) ([]string, bool) {
	t.Helper()
	if _, err := os.Stat(r.dir); err != nil {
		if os.Getenv(r.requireEnv) != "" {
			t.Fatalf("%s is absent but %s is set; fetch it with %s", r.name, r.requireEnv, r.fetch)
		}
		if r.requireEnv == "" {
			t.Fatalf("%s: %v", r.name, err)
		}
		t.Logf("skipping %s: absent; fetch it with %s", r.name, r.fetch)
		return nil, false
	}
	var files []string
	err := filepath.WalkDir(r.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.dir, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			for _, within := range r.within {
				if rel == within {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if source.KindOf(path) != source.KindUnknown {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", r.name, err)
	}
	sort.Strings(files)
	return files, true
}

// readCorpusRoundTripExpected returns the recorded outcome per path and the
// recorded size of each root.
func readCorpusRoundTripExpected(t *testing.T) (map[string]string, map[string]int) {
	t.Helper()
	content, err := os.ReadFile(corpusRoundTripExpected)
	if err != nil {
		t.Fatalf("read %s: %v", corpusRoundTripExpected, err)
	}
	want := make(map[string]string)
	totals := make(map[string]int)
	for i, line := range strings.Split(string(content), "\n") {
		if rest, ok := strings.CutPrefix(line, "# files: "); ok {
			root, count, found := strings.Cut(rest, " ")
			if !found {
				t.Fatalf("%s:%d: want \"# files: <root> <n>\", got %q", corpusRoundTripExpected, i+1, line)
			}
			n, err := strconv.Atoi(count)
			if err != nil {
				t.Fatalf("%s:%d: bad file count: %v", corpusRoundTripExpected, i+1, err)
			}
			totals[root] = n
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		outcome, path, found := strings.Cut(line, "\t")
		if !found {
			t.Fatalf("%s:%d: want \"<outcome>\\t<path>\", got %q", corpusRoundTripExpected, i+1, line)
		}
		want[path] = outcome
	}
	return want, totals
}

// writeCorpusRoundTripExpected records each root's size and every path that is
// not byte-stable, sorted by path.
func writeCorpusRoundTripExpected(t *testing.T, counts map[string]int, got map[string]string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(corpusRoundTripHeader)
	for _, root := range corpusRoots {
		fmt.Fprintf(&b, "# files: %s %d\n", root.name, counts[root.name])
	}
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s\t%s\n", got[key], key)
	}
	if err := os.WriteFile(corpusRoundTripExpected, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write %s: %v", corpusRoundTripExpected, err)
	}
	t.Logf("wrote %s", corpusRoundTripExpected)
}
