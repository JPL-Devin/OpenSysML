package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestManualWorkedExampleMatchesCommittedOutput renders the user manual's
// worked example (docs/manual/examples/observatory.sysml) through the binary
// and compares it against the committed rendered output, so the manual's
// "full rendered output" can never drift from what the binary produces.
func TestManualWorkedExampleMatchesCommittedOutput(t *testing.T) {
	binary := buildCLI(t)
	examples := filepath.Join("..", "..", "docs", "manual", "examples")
	source := filepath.Join(examples, "observatory.sysml")
	committed, err := os.ReadFile(filepath.Join(examples, "observatory.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "observatory.md")
	cmd := exec.Command(binary, source, "-render-document", "Observatory::MassReport", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(committed) {
		t.Errorf("rendered manual example differs from docs/manual/examples/observatory.md:\n%s", written)
	}
}

// TestManualCookbookModelAnalysesCleanly parses and analyses the manual's
// query-cookbook model, so every recipe the manual quotes keeps compiling.
func TestManualCookbookModelAnalysesCleanly(t *testing.T) {
	binary := buildCLI(t)
	source := filepath.Join("..", "..", "docs", "manual", "examples", "cookbook.sysml")
	cmd := exec.Command(binary, source, "-run-query", "Cookbook::MassTable root=Cookbook::telescope")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cookbook query: %v\n%s", err, output)
	}
}
