package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doccounts"
)

const (
	fixtureCompliance = `# Compliance

**No external referee:** self-assessed.

The map below tracks 99 semantic rules: **99 ✅ faithful, 0 ⚠️ approximate, 0 ❌ not implemented, 0 ⛔ deliberate divergence.**

| Rule | Status |
|---|---|
| a | ✅ Faithful |
| b | ⚠️ Approximate |
`
	fixtureBookkeeping = `# Guide

**Reference differential:** 99 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (` + "`" + `old` + "`" + `), 99 in full agreement;
**Rejection oracle:** the reverse direction — do we reject what the reference rejects? 99 hand-written invalid models validated by both implementations, 99 rejected by both, 99 the pinned pilot rejects and we accept;
<!-- doc-counts:begin refereed-figures -->
old generated block
<!-- doc-counts:end refereed-figures -->
Nothing else on this line's neighbours moves.
`
	fixtureDifferentialBaseline = `{"pilotRelease":"2026-05 (jupyter-sysml-kernel 0.60.1)","totals":{"files":2,"filesFullyAgreeing":1,"openSysMLOnly":3,"pilotOnly":4},` +
		`"errata":{"registryEntries":2,"corrections":1,"documentedWithoutCorrection":1,"totals":{"files":2,"filesFullyAgreeing":2,"openSysMLOnly":2,"pilotOnly":4}}}`
	fixtureXpectBaseline = `{"kinds":[{"kind":"errors","assertions":2,"rows":2,"agree":2,"wordingOnly":1,"sameLocation":0,"sameLine":0,"severityDiffers":0,"elsewhereInFile":0},{"kind":"scope","assertions":3,"agree":2}],` +
		`"errata":{"kinds":[{"kind":"errors","assertions":2,"rows":2,"agree":2,"wordingOnly":1}]}}`
	fixtureRejectionBaseline = `{"totals":{"cases":2,"bothReject":2,"pilotOnlyRejects":0},"strictOnlyAgreements":[],` +
		`"errata":{"totals":{"cases":2,"bothReject":2,"pilotOnlyRejects":0}}}`
)

// TestRunRewritesEveryDerivedLineAndIsIdempotent is the guarantee the wave-9
// workflow rests on: one command, byte-identical output, second run a no-op.
func TestRunRewritesEveryDerivedLineAndIsIdempotent(t *testing.T) {
	root := writeFixture(t)

	rewritten, err := run(root, io.Discard)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if rewritten != 3 {
		t.Fatalf("first run rewrote %d files, want 3", rewritten)
	}
	first := map[string]string{}
	for _, path := range []string{doccounts.SpecCompliancePath, doccounts.ReadmePath, doccounts.ArchitecturePath} {
		first[path] = read(t, root, path)
	}
	if want := "The map below tracks 2 semantic rules: **1 ✅ faithful, 1 ⚠️ approximate, 0 ❌ not implemented, 0 ⛔ deliberate divergence.**"; !strings.Contains(first[doccounts.SpecCompliancePath], want) {
		t.Fatalf("header not restated:\n%s", first[doccounts.SpecCompliancePath])
	}
	for _, path := range []string{doccounts.ReadmePath, doccounts.ArchitecturePath} {
		if !strings.Contains(first[path], "status of each of the 2 tracked rules stays in [spec compliance]") {
			t.Fatalf("%s bookkeeping line not restated:\n%s", path, first[path])
		}
		if !strings.Contains(first[path], "Nothing else on this line's neighbours moves.") {
			t.Fatalf("%s lost a neighbouring line", path)
		}
	}

	rewritten, err = run(root, io.Discard)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if rewritten != 0 {
		t.Fatalf("second run rewrote %d files, want 0", rewritten)
	}
	for path, content := range first {
		if read(t, root, path) != content {
			t.Fatalf("%s changed on the second run", path)
		}
	}
}

// TestRunWritesNothingWhenALaterFileCannotBeRewritten keeps the tree consistent:
// a partly restated tree would state two different censuses at once.
func TestRunWritesNothingWhenALaterFileCannotBeRewritten(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, doccounts.ArchitecturePath, "**Row bookkeeping:** reworded, and no longer the line the pattern states.\n")
	before := read(t, root, doccounts.SpecCompliancePath)

	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error for a derived line the pattern does not match")
	}
	if read(t, root, doccounts.SpecCompliancePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
	}
}

func TestRunWritesNothingWhenAFileIsNotWritable(t *testing.T) {
	root := writeFixture(t)
	readonly := filepath.Join(root, filepath.FromSlash(doccounts.ReadmePath))
	if err := os.Chmod(readonly, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	before := read(t, root, doccounts.SpecCompliancePath)

	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error for a file that cannot be written")
	}
	if read(t, root, doccounts.SpecCompliancePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
	}
}

func TestRunReportsAMapWithNoRuleRows(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, doccounts.SpecCompliancePath, "# Compliance\n\nThe map below tracks 0 semantic rules: **0 ✅ faithful, 0 ⚠️ approximate, 0 ❌ not implemented, 0 ⛔ deliberate divergence.**\n")
	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error when the compliance map states no rule rows")
	}
}

func TestCheckReportsStaleFilesWithoutWriting(t *testing.T) {
	root := writeFixture(t)
	before := read(t, root, doccounts.ReadmePath)
	var output strings.Builder
	stale, err := check(root, &output)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stale != 3 {
		t.Fatalf("check reported %d stale files, want 3", stale)
	}
	if !strings.Contains(output.String(), "README.md is stale") {
		t.Fatalf("check report does not name README.md:\n%s", output.String())
	}
	if read(t, root, doccounts.ReadmePath) != before {
		t.Fatal("check mode changed README.md")
	}
}

func TestCheckCommittedTreeIsCurrent(t *testing.T) {
	var output strings.Builder
	stale, err := check("../..", &output)
	if err != nil {
		t.Fatalf("check committed tree: %v", err)
	}
	if stale != 0 {
		t.Fatalf("committed tree has %d stale files:\n%s", stale, output.String())
	}
}

func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeAt(t, root, doccounts.SpecCompliancePath, fixtureCompliance)
	writeAt(t, root, doccounts.ReadmePath, fixtureBookkeeping)
	writeAt(t, root, doccounts.ArchitecturePath, fixtureBookkeeping)
	writeAt(t, root, "docs/project/pilot-differential-baseline.json", fixtureDifferentialBaseline)
	writeAt(t, root, "docs/project/pilot-xpect-baseline.json", fixtureXpectBaseline)
	writeAt(t, root, "docs/project/pilot-rejection-baseline.json", fixtureRejectionBaseline)
	return root
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

func read(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
