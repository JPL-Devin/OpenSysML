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

The map below tracks 99 semantic rules: **99 ✅ faithful, 0 ⚠️ approximate, 0 ❌ not implemented, 0 ⛔ deliberate divergence.**

| Rule | Status |
|---|---|
| a | ✅ Faithful |
| b | ⚠️ Approximate |
`
	fixtureBookkeeping = `# Guide

**Row bookkeeping:** the ✅/⚠️/❌/⛔ status of each of the 99 tracked rules stays in the map.
Nothing else on this line's neighbours moves.
`
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
		if !strings.Contains(first[path], "each of the 2 tracked rules stays in the map.") {
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
		if got := read(t, root, path); got != content {
			t.Fatalf("%s changed on the second run", path)
		}
	}
}

func TestRunReportsAMapWithNoRuleRows(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, doccounts.SpecCompliancePath, "# Compliance\n\nThe map below tracks 0 semantic rules: **0 ✅ faithful, 0 ⚠️ approximate, 0 ❌ not implemented, 0 ⛔ deliberate divergence.**\n")
	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error when the compliance map states no rule rows")
	}
}

func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeAt(t, root, doccounts.SpecCompliancePath, fixtureCompliance)
	writeAt(t, root, doccounts.ReadmePath, fixtureBookkeeping)
	writeAt(t, root, doccounts.ArchitecturePath, fixtureBookkeeping)
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
