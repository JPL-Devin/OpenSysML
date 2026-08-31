package doccounts

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestReadRefereedCountsDerivesAllBaselineFigures(t *testing.T) {
	root := t.TempDir()
	writeDoccountsFixture(t, root)
	counts, err := ReadRefereedCounts(root)
	if err != nil {
		t.Fatalf("read baselines: %v", err)
	}
	if counts.Files != 2 || counts.FilesAgreeing != 1 || counts.OursOnly != 3 || counts.PilotOnly != 4 {
		t.Fatalf("differential counts: %+v", counts)
	}
	if counts.DeclaredErrors != 11 || counts.Silent != 1 || counts.DeclaredAgree != 6 ||
		counts.WordingOnly != 2 || counts.LocationOnly != 1 || counts.SeverityDiffers != 0 ||
		counts.Elsewhere != 0 || counts.ScopeExact != 7 || counts.ScopeTotal != 8 {
		t.Fatalf("Xpect counts: %+v", counts)
	}
	if counts.RejectCases != 12 || counts.RejectBoth != 11 || counts.RejectPilotOnly != 1 ||
		counts.RejectDefaultBoth != 9 || counts.RejectDefaultPilotOnly != 3 || counts.RejectStrictOnly != 2 {
		t.Fatalf("rejection counts: %+v", counts)
	}
	if counts.SelfAssessed != 1 || counts.PilotTag != "2026-05" || counts.PilotArtifact != "0.60.1" {
		t.Fatalf("derived metadata: %+v", counts)
	}
	want := ErrataCounts{Registry: 2, Corrections: 1, Documented: 1,
		Files: 2, FilesAgreeing: 2, OursOnly: 2, PilotOnly: 4, Silent: 0, RejectCases: 12, RejectPilotOnly: 1}
	if counts.Errata != want {
		t.Fatalf("errata counts: %+v, want %+v", counts.Errata, want)
	}
}

// TestReadRefereedCountsRejectsABaselineWithoutErrata keeps the second figure a
// measurement: a baseline predating the overlay is stale, not zero.
func TestReadRefereedCountsRejectsABaselineWithoutErrata(t *testing.T) {
	for _, path := range []string{
		"docs/project/pilot-differential-baseline.json",
		"docs/project/pilot-xpect-baseline.json",
		"docs/project/pilot-rejection-baseline.json",
	} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			writeDoccountsFixture(t, root)
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(content, &decoded); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			delete(decoded, "errata")
			stripped, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("encode fixture: %v", err)
			}
			writeAt(t, root, path, string(stripped))
			if _, err := ReadRefereedCounts(root); err == nil {
				t.Fatalf("%s without an errata section: want an error", path)
			}
		})
	}
}

func TestRewriteBlockUsesConsumerRelativeLinksAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	writeDoccountsFixture(t, root)
	counts, err := ReadRefereedCounts(root)
	if err != nil {
		t.Fatalf("read baselines: %v", err)
	}
	spec := Block{Path: "README.md", Name: "refereed-figures", LinkPrefix: "docs/project/"}
	content := "before\n<!-- doc-counts:begin refereed-figures -->\nstale\n<!-- doc-counts:end refereed-figures -->\nafter\n"
	got, err := RewriteBlock(content, spec, counts)
	if err != nil {
		t.Fatalf("rewrite block: %v", err)
	}
	for _, want := range []string{
		"`PILOT_TAG=2026-05`", "artifact `0.60.1`", "1 of 2 files",
		"6 we report word-for-word", "9 both reject",
		"[differential](docs/project/pilot-differential.md)",
		"[spec compliance](docs/project/spec-compliance.md)",
		"must be read by root",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated block lacks %q:\n%s", want, got)
		}
	}
	again, err := RewriteBlock(got, spec, counts)
	if err != nil {
		t.Fatalf("second block rewrite: %v", err)
	}
	if again != got {
		t.Fatal("block rewrite is not idempotent")
	}
}

// TestRewriteBlockRendersTheLandingBandFromTheSameCensus keeps the landing page's
// figures the generated ones: the band states the same census as the prose block,
// and its links go through the site's record() global, which resolves each record to
// its page or to the repository, and which the site build checks.
func TestRewriteBlockRendersTheLandingBandFromTheSameCensus(t *testing.T) {
	root := t.TempDir()
	writeDoccountsFixture(t, root)
	counts, err := ReadRefereedCounts(root)
	if err != nil {
		t.Fatalf("read baselines: %v", err)
	}
	spec := Block{Path: LandingPath, Name: landingBlockName, LinkPrefix: "project/"}
	content := "<section>\n    <!-- doc-counts:begin landing-figures -->\n    stale\n    <!-- doc-counts:end landing-figures -->\n</section>\n"
	got, err := RewriteBlock(content, spec, counts)
	if err != nil {
		t.Fatalf("rewrite landing block: %v", err)
	}
	for _, want := range []string{
		"<code>2026-05</code>", "artifact <code>0.60.1</code>",
		">1 of 2<", ">1 of 11<", ">7 of 8<", ">3 of 12<",
		"the 1 behavioral rules",
		"href=\"{{ record('project/pilot-differential.md', base_url) }}\"",
		"href=\"{{ record('project/spec-compliance.md', base_url) }}\"",
		"href=\"{{ record('project/pilot-xpect.md', base_url) }}\"",
		"href=\"{{ record('project/pilot-rejection.md', base_url) }}\"",
		"<section>", "</section>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("landing band lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "stale") {
		t.Fatal("landing band kept the stale content")
	}
	again, err := RewriteBlock(got, spec, counts)
	if err != nil {
		t.Fatalf("second landing rewrite: %v", err)
	}
	if again != got {
		t.Fatal("landing block rewrite is not idempotent")
	}
}

// TestRewriteBlockRejectsABlockWithNoTemplate keeps a new consumer from silently
// emptying a block: a name no template renders is an error, not empty markup.
func TestRewriteBlockRejectsABlockWithNoTemplate(t *testing.T) {
	content := "<!-- doc-counts:begin invented -->\nkept\n<!-- doc-counts:end invented -->\n"
	if _, err := RewriteBlock(content, Block{Path: ReadmePath, Name: "invented"}, RefereedCounts{}); err == nil {
		t.Fatal("want an error for a block name no template renders")
	}
}

func TestRewriteBlockRejectsMalformedMarkers(t *testing.T) {
	spec := Block{Path: "README.md", Name: "refereed-figures"}
	counts := RefereedCounts{}
	for name, content := range map[string]string{
		"missing begin": "<!-- doc-counts:end refereed-figures -->\n",
		"missing end":   "<!-- doc-counts:begin refereed-figures -->\n",
		"reversed":      "<!-- doc-counts:end refereed-figures -->\n<!-- doc-counts:begin refereed-figures -->\n",
		"duplicate":     "<!-- doc-counts:begin refereed-figures -->\n<!-- doc-counts:begin refereed-figures -->\n<!-- doc-counts:end refereed-figures -->\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := RewriteBlock(content, spec, counts); err == nil {
				t.Fatal("want malformed marker error")
			}
		})
	}
}

func TestRewriteBaselineLineRestatesOnlyCapturedValues(t *testing.T) {
	spec := BaselineLines()[0]
	content := "before\n**Reference differential:** 99 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (`old`), 99 in full agreement;\nafter\n"
	counts := RefereedCounts{Files: 2, PilotTag: "2026-05", FilesAgreeing: 1}
	got, err := RewriteBaselineLine(content, spec, counts)
	if err != nil {
		t.Fatalf("rewrite baseline line: %v", err)
	}
	want := "before\n**Reference differential:** 2 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (`2026-05`), 1 in full agreement;\nafter\n"
	if got != want {
		t.Fatalf("rewritten baseline line:\n%s", got)
	}
}

func TestReadSelfAssessedRowsRequiresASection(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, SpecCompliancePath, "# Compliance\n\n| Rule | Status |\n|---|---|\n| a | ✅ |\n")
	if _, err := ReadSelfAssessedRows(root); err == nil {
		t.Fatal("want an error when no section declares no external referee")
	}
}

func writeDoccountsFixture(t *testing.T, root string) {
	t.Helper()
	writeAt(t, root, SpecCompliancePath, `# Compliance

**No external referee:** self-assessed.

| Rule | Status |
|---|---|
| a | ✅ Faithful |
`)
	writeAt(t, root, "docs/project/pilot-differential-baseline.json", `{"pilotRelease":"2026-05 (jupyter-sysml-kernel 0.60.1)","totals":{"files":2,"filesFullyAgreeing":1,"openSysMLOnly":3,"pilotOnly":4},`+
		`"errata":{"registryEntries":2,"corrections":1,"documentedWithoutCorrection":1,"totals":{"files":2,"filesFullyAgreeing":2,"openSysMLOnly":2,"pilotOnly":4}}}`)
	writeAt(t, root, "docs/project/pilot-xpect-baseline.json", `{"kinds":[{"kind":"errors","rows":11,"agree":8,"wordingOnly":2,"sameLocation":1,"sameLine":1,"severityDiffers":0,"elsewhereInFile":0},{"kind":"scope","assertions":8,"agree":7}],`+
		`"errata":{"kinds":[{"kind":"errors","rows":11,"agree":9,"wordingOnly":2,"sameLocation":1,"sameLine":1}]}}`)
	writeAt(t, root, "docs/project/pilot-rejection-baseline.json", `{"totals":{"cases":12,"bothReject":11,"pilotOnlyRejects":1},"strictOnlyAgreements":["a","b"],`+
		`"errata":{"totals":{"cases":12,"bothReject":11,"pilotOnlyRejects":1}}}`)
}

func writeAt(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
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
