package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
)

// TestCensusIsCurrent is the gate in test form: the committed baseline, the
// census document and the probes must agree, and the baseline must list what
// the pinned jar contains whenever the jar is provisioned.
func TestCensusIsCurrent(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCheck(root, options{}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

// TestExtractionMatchesBaseline compares a fresh extraction with the committed
// baseline when the jar is provisioned, so a stale baseline fails here too.
func TestExtractionMatchesBaseline(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := baseline.ReadPin(root)
	if err != nil {
		t.Fatal(err)
	}
	jar, present, err := options{}.jarPath(root, pin)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Skipf("pinned jar not provisioned at %s", jar)
	}
	extracted, err := extractFromJar(jar)
	if err != nil {
		t.Fatal(err)
	}
	base, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := base.matches(extracted); err != nil {
		t.Fatal(err)
	}
	if len(extracted) != len(base.Constraints) {
		t.Fatalf("extracted %d constraints, baseline records %d", len(extracted), len(base.Constraints))
	}
}

// TestRecordedDateFollowsContent: an unchanged baseline keeps its date, a
// changed one is stamped today, and a first recording is dated.
func TestRecordedDateFollowsContent(t *testing.T) {
	previous := testBaseline(Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful})
	previous.Recorded = "2020-01-01"
	same := testBaseline(Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful})
	if got := recordedDate(previous, same); got != "2020-01-01" {
		t.Errorf("unchanged baseline re-dated to %q", got)
	}
	changed := testBaseline(Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful}, Constraint{Name: "validateB", Source: "sysml", Status: StatusUnknown})
	if got := recordedDate(previous, changed); got != baseline.Today() {
		t.Errorf("changed baseline dated %q, want today", got)
	}
	if got := recordedDate(nil, same); got != baseline.Today() {
		t.Errorf("first recording dated %q, want today", got)
	}
}

// TestUpdateIsIdempotentAcrossDays runs -update over a copy of the committed
// baseline carrying an old date; with the jar present it must not change a byte.
func TestUpdateIsIdempotentAcrossDays(t *testing.T) {
	repo, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := baseline.ReadPin(repo)
	if err != nil {
		t.Fatal(err)
	}
	jar, present, err := options{}.jarPath(repo, pin)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Skipf("pinned jar not provisioned at %s", jar)
	}
	root := t.TempDir()
	for _, rel := range []string{baseline.PinPath, baselinePath} {
		content, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	base.Recorded = "2020-01-01"
	if err := writeBaseline(root, base); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	if err := runUpdate(root, options{jar: jar}, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("-update over an unchanged extraction rewrote the baseline:\n%s", after)
	}
}

// TestStringConstantsReadsThePool builds a class file whose pool holds one
// string constant and one bare Utf8 entry, and expects only the former.
func TestStringConstantsReadsThePool(t *testing.T) {
	var pool bytes.Buffer
	write := func(v any) {
		if err := binary.Write(&pool, binary.BigEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	write(uint32(0xCAFEBABE))
	write(uint16(0))
	write(uint16(65))
	write(uint16(6)) // constant_pool_count: entries 1..5
	// 1: Utf8 "validateFoo_"
	write(uint8(1))
	write(uint16(len("validateFoo_")))
	pool.WriteString("validateFoo_")
	// 2: String -> 1
	write(uint8(8))
	write(uint16(1))
	// 3: Utf8 "validateBareMethodName" (not a string constant)
	write(uint8(1))
	write(uint16(len("validateBareMethodName")))
	pool.WriteString("validateBareMethodName")
	// 4: Long (two slots)
	write(uint8(5))
	write(uint64(7))
	got, err := stringConstants(pool.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "validateFoo_" {
		t.Fatalf("string constants = %q, want [validateFoo_]", got)
	}
	if _, err := stringConstants([]byte{1, 2, 3}); err == nil {
		t.Fatal("a non-class file must be rejected")
	}
}

// testProvenance are the provenance lines of a census document whose baseline
// is testBaseline.
const testProvenance = "**Pilot:** [Pilot](https://example.test) release `2026-07`, commit `c7fc737d`, artifact `jupyter-sysml-kernel 0.61.0` — the pin\n" +
	"**Jar:** `kernel-0.61.0-all.jar` (`sha256:abc`), provisioned by a script\n"

func testBaseline(constraints ...Constraint) *Baseline {
	return &Baseline{
		PilotTag: "2026-07", PilotCommit: "c7fc737d", PilotArtifact: "0.61.0",
		Jar:         JarRecord{Name: "kernel-0.61.0-all.jar", Digest: "sha256:abc"},
		Constraints: constraints,
	}
}

// TestRewriteDerivedLinesRestatesTheSummary checks the provenance and summary
// lines are rewritten from the baseline and nothing else moves.
func TestRewriteDerivedLinesRestatesTheSummary(t *testing.T) {
	base := testBaseline(
		Constraint{Name: "validateA", Status: StatusFaithful},
		Constraint{Name: "validateB", Status: StatusApproximate},
		Constraint{Name: "validateC", Status: StatusNotImplemented},
		Constraint{Name: "validateD", Status: StatusUnknown},
	)
	stale := "**Pilot:** [Pilot](https://example.test) release `2025-01`, commit `old`, artifact `jupyter-sysml-kernel 0.50.0` — the pin\n" +
		"**Jar:** `kernel-0.50.0-all.jar` (`sha256:old`), provisioned by a script\n"
	content := "# Census\n\n" + stale + "\n**Census:** 0 of 0 named constraints are reported by OpenSysML — 0 ✅ faithful and 0 ⚠️ approximate; 0 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 0 ❔ unknown.\n\ntrailing\n"
	got, err := rewriteDerivedLines(content, base)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Census\n\n" + testProvenance + "\n**Census:** 2 of 4 named constraints are reported by OpenSysML — 1 ✅ faithful and 1 ⚠️ approximate; 1 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 1 ❔ unknown.\n\ntrailing\n"
	if got != want {
		t.Fatalf("rewrite:\n%s\nwant:\n%s", got, want)
	}
	if _, err := rewriteDerivedLines(testProvenance+"no summary here\n", base); err == nil {
		t.Fatal("a document without the summary line must be rejected")
	}
	if _, err := rewriteDerivedLines("**Pilot:** unrecognised\n", base); err == nil {
		t.Fatal("a provenance line the pattern does not match must be rejected")
	}
}

// TestCheckDocumentRejectsDrift covers the three drifts the gate exists for:
// a row the baseline lacks, a baseline name without a row, a hand-edited figure.
func TestCheckDocumentRejectsDrift(t *testing.T) {
	root := t.TempDir()
	corpus := filepath.Join(root, filepath.FromSlash(negativeCorpusDir))
	if err := os.MkdirAll(filepath.Join(corpus, "kerml", "dir.kerml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpus, "kerml", "a.kerml"), []byte("package P;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := testBaseline(
		Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful},
		Constraint{Name: "validateB", Source: "sysml", Status: StatusNotImplemented},
	)
	doc := testProvenance + strings.Join([]string{
		"**Census:** 1 of 2 named constraints are reported by OpenSysML — 1 ✅ faithful and 0 ⚠️ approximate; 1 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 0 ❔ unknown.",
		"",
		"| Constraint | Language | Checks | Implementation | Our message | Negative case | Status |",
		"|---|---|---|---|---|---|---|",
		"| `validateA` | KerML | a | `x.go:f` | same | `kerml/a.kerml` | ✅ faithful |",
		"| `validateB` | SysML | b | — | — | none | ❌ not implemented |",
		"",
	}, "\n")
	if err := checkDocument(root, doc, base); err != nil {
		t.Fatalf("a consistent document must pass: %v", err)
	}
	cases := map[string]struct {
		mutate func(string) string
		want   string
	}{
		"extra row": {
			mutate: func(s string) string {
				row := "| `validateB` | SysML | b | — | — | none | ❌ not implemented |\n"
				return strings.Replace(s, row, row+"| `validateC` | SysML | c | — | — | none | ❌ not implemented |\n", 1)
			},
			want: "validateC is in the table but not in",
		},
		"missing row": {
			mutate: func(s string) string {
				return strings.Replace(s, "| `validateB` | SysML | b | — | — | none | ❌ not implemented |\n", "", 1)
			},
			want: "validateB is in docs/project/validation-constraints-baseline.json but has no row",
		},
		"hand-edited figure": {
			mutate: func(s string) string { return strings.Replace(s, "1 of 2", "2 of 2", 1) },
			want:   "line is stale",
		},
		"stale release": {
			mutate: func(s string) string { return strings.Replace(s, "release `2026-07`", "release `2025-01`", 1) },
			want:   "line is stale",
		},
		"stale commit": {
			mutate: func(s string) string { return strings.Replace(s, "commit `c7fc737d`", "commit `deadbeef`", 1) },
			want:   "line is stale",
		},
		"stale artifact": {
			mutate: func(s string) string { return strings.Replace(s, "kernel 0.61.0`", "kernel 0.62.0`", 1) },
			want:   "line is stale",
		},
		"stale jar name": {
			mutate: func(s string) string {
				return strings.Replace(s, "`kernel-0.61.0-all.jar`", "`kernel-0.62.0-all.jar`", 1)
			},
			want: "line is stale",
		},
		"stale digest": {
			mutate: func(s string) string { return strings.Replace(s, "`sha256:abc`", "`sha256:def`", 1) },
			want:   "line is stale",
		},
		"status disagrees": {
			mutate: func(s string) string { return strings.Replace(s, "| ✅ faithful |", "| ⚠️ approximate |", 1) },
			want:   "the baseline records faithful",
		},
		"language disagrees": {
			mutate: func(s string) string {
				return strings.Replace(s, "| `validateA` | KerML |", "| `validateA` | SysML |", 1)
			},
			want: "has language \"SysML\"",
		},
		"missing negative case": {
			mutate: func(s string) string { return strings.Replace(s, "`kerml/a.kerml`", "`kerml/gone.kerml`", 1) },
			want:   "does not exist",
		},
		"negative case is a directory": {
			mutate: func(s string) string { return strings.Replace(s, "`kerml/a.kerml`", "`kerml/dir.kerml`", 1) },
			want:   "is not a file",
		},
		"negative case is not a model": {
			mutate: func(s string) string { return strings.Replace(s, "`kerml/a.kerml`", "`kerml`", 1) },
			want:   "is not a .sysml or .kerml file",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := checkDocument(root, tc.mutate(doc), base)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestCheckProbesCoverage covers what the probe check demands: a probe for every
// implemented row, one per notation when both validators declare the
// constraint, and none for a row recorded as unreported.
func TestCheckProbesCoverage(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(probesDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, constraint string) {
		content := "// Census: " + constraint + "\n// Expect: error: x\npackage P;\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := &Baseline{Constraints: []Constraint{
		{Name: "validateA", Source: "kerml", Status: StatusFaithful},
		{Name: "validateB", Source: "both", Status: StatusApproximate},
		{Name: "validateC", Source: "sysml", Status: StatusNotImplemented},
	}}
	write("validateA.sysml", "validateA")
	write("validateB.kerml", "validateB")
	err := checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateB is recorded approximate in both notations but no .sysml probe") {
		t.Fatalf("a shared constraint with one notation probed must fail, got %v", err)
	}
	write("validateB.sysml", "validateB")
	if err := checkProbes(root, base); err != nil {
		t.Fatalf("both notations probed must pass: %v", err)
	}
	write("validateC.sysml", "validateC")
	err = checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateC, which the baseline records as not-implemented") {
		t.Fatalf("a probe for an unreported constraint must fail, got %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "validateC.sysml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "validateA.sysml")); err != nil {
		t.Fatal(err)
	}
	err = checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateA is recorded faithful but no probe") {
		t.Fatalf("an implemented row without a probe must fail, got %v", err)
	}
}

// TestBaselineMatchesNamesEverySide checks the jar comparison names a removed and
// an added constraint.
func TestBaselineMatchesNamesEverySide(t *testing.T) {
	base := &Baseline{Constraints: []Constraint{
		{Name: "validateA", Source: "kerml", Status: StatusUnknown},
		{Name: "validateB", Source: "sysml", Raw: "validateB_", Status: StatusUnknown},
	}}
	if err := base.matches([]Extracted{{Name: "validateA", Source: "kerml"}, {Name: "validateB", Raw: "validateB_", Source: "sysml"}}); err != nil {
		t.Fatal(err)
	}
	err := base.matches([]Extracted{{Name: "validateA", Source: "kerml"}, {Name: "validateC", Source: "sysml"}})
	if err == nil {
		t.Fatal("a differing list must be rejected")
	}
	for _, want := range []string{"validateB is in the baseline but not in the jar", "validateC is in the jar but not in the baseline"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
