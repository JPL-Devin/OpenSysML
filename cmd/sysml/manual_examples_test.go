package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestManualWorkedExampleMatchesCommittedOutput renders the manual's worked
// example and compares it against the committed output, so the two never drift.
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
	for _, query := range []string{
		"Cookbook::MassTable root=Cookbook::telescope",
		"Cookbook::MassBudget root=Cookbook::telescope",
	} {
		cmd := exec.Command(binary, source, "-run-query", query)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cookbook query %s: %v\n%s", query, err, output)
		}
	}
}
