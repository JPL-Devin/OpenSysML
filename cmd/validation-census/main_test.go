package main

import (
	"bytes"
	"encoding/binary"
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

// TestRewriteDerivedLinesRestatesTheSummary checks the summary line is rewritten
// from the baseline and nothing else moves.
func TestRewriteDerivedLinesRestatesTheSummary(t *testing.T) {
	base := &Baseline{Constraints: []Constraint{
		{Name: "validateA", Status: StatusFaithful},
		{Name: "validateB", Status: StatusApproximate},
		{Name: "validateC", Status: StatusNotImplemented},
		{Name: "validateD", Status: StatusUnknown},
	}}
	content := "# Census\n\n**Census:** 0 of 0 named constraints are reported by OpenSysML — 0 ✅ faithful and 0 ⚠️ approximate; 0 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 0 ❔ unknown.\n\ntrailing\n"
	got, err := rewriteDerivedLines(content, base)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Census\n\n**Census:** 2 of 4 named constraints are reported by OpenSysML — 1 ✅ faithful and 1 ⚠️ approximate; 1 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 1 ❔ unknown.\n\ntrailing\n"
	if got != want {
		t.Fatalf("rewrite:\n%s\nwant:\n%s", got, want)
	}
	if _, err := rewriteDerivedLines("no summary here\n", base); err == nil {
		t.Fatal("a document without the summary line must be rejected")
	}
}

// TestCheckDocumentRejectsDrift covers the three drifts the gate exists for:
// a row the baseline lacks, a baseline name without a row, a hand-edited figure.
func TestCheckDocumentRejectsDrift(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(negativeCorpusDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	base := &Baseline{Constraints: []Constraint{
		{Name: "validateA", Source: "kerml", Status: StatusFaithful},
		{Name: "validateB", Source: "sysml", Status: StatusNotImplemented},
	}}
	doc := strings.Join([]string{
		"**Census:** 1 of 2 named constraints are reported by OpenSysML — 1 ✅ faithful and 0 ⚠️ approximate; 1 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 0 ❔ unknown.",
		"",
		"| Constraint | Language | Checks | Implementation | Our message | Negative case | Status |",
		"|---|---|---|---|---|---|---|",
		"| `validateA` | KerML | a | `x.go:f` | same | none | ✅ faithful |",
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
			mutate: func(s string) string { return strings.Replace(s, "| none | ✅", "| `kerml/gone.kerml` | ✅", 1) },
			want:   "does not exist",
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
