package doccounts

import (
	"strings"
	"testing"
)

const complianceFixture = `# Compliance

The map below tracks 1 semantic rules: **1 ✅ faithful, 0 ⚠️ approximate, 0 ❌ not implemented, 0 ⛔ deliberate divergence.**

| Rule | Where | Test | Status |
|---|---|---|---|
| a | x | y | ✅ Faithful |
| b | x | y | ⚠️ Approximate |
| c | x | y | ❌ Not implemented |
| d | x | y | ⛔ Deliberate |
| notes | mentions ✅ and ❌ together | y | ⚠️ Approximate |
`

func TestCountRulesCountsOneMarkerPerRow(t *testing.T) {
	counts := CountRules(complianceFixture)
	want := RuleCounts{Total: 4, Faithful: 1, Approximate: 1, NotImplemented: 1, Deliberate: 1}
	if counts != want {
		t.Fatalf("census: want %+v, got %+v", want, counts)
	}
}

func TestCountRulesIgnoresProseAndHeaders(t *testing.T) {
	if counts := CountRules("✅ prose outside a table\n\n| header | Status |\n|---|---|\n"); counts.Total != 0 {
		t.Fatalf("census of a table with no rule rows: want 0 rows, got %+v", counts)
	}
}

func TestRewriteRestatesTheHeaderAndNothingElse(t *testing.T) {
	spec := Lines()[0]
	got, err := Rewrite(complianceFixture, spec, CountRules(complianceFixture))
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !strings.Contains(got, "The map below tracks 4 semantic rules: **1 ✅ faithful, 1 ⚠️ approximate, 1 ❌ not implemented, 1 ⛔ deliberate divergence.**") {
		t.Fatalf("rewritten header not found in:\n%s", got)
	}
	if stripLine(got, "The map below tracks") != stripLine(complianceFixture, "The map below tracks") {
		t.Fatal("rewrite changed a line other than the derived one")
	}
	again, err := Rewrite(got, spec, CountRules(got))
	if err != nil {
		t.Fatalf("second rewrite: %v", err)
	}
	if again != got {
		t.Fatal("rewrite is not idempotent")
	}
}

func TestRewriteReportsAKnownFailureRow(t *testing.T) {
	content := strings.Replace(complianceFixture, "| d | x | y | ⛔ Deliberate |", "| d | x | y | 🚧 Known failure |", 1)
	if _, err := Rewrite(content, Lines()[0], CountRules(content)); err == nil {
		t.Fatal("a 🚧 row has no place in the derived lines; want an error")
	}
}

func TestRewriteReportsProseItDoesNotRecognise(t *testing.T) {
	content := strings.Replace(complianceFixture, "The map below tracks 1 semantic rules:", "The map below tracks some semantic rules:", 1)
	if _, err := Rewrite(content, Lines()[0], CountRules(content)); err == nil {
		t.Fatal("want an error for a marker line the pattern does not match")
	}
	if _, err := Rewrite("no marker here\n", Lines()[0], RuleCounts{Total: 1}); err == nil {
		t.Fatal("want an error when no line carries the marker")
	}
}

// stripLine returns content without the line carrying the marker, so a rewrite
// can be compared everywhere else.
func stripLine(content, marker string) string {
	var kept []string
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, marker) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
