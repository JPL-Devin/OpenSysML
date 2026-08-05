package model

import (
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"os"
	"testing"
)

func TestRequirementDefinitionsFile(t *testing.T) {
	path := "../../../examples/sysml-v2-training/32. Requirements/Requirement Definitions.sysml"

	// Skip if training examples not downloaded
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Training examples not downloaded (run ./scripts/download-training-examples.sh)")
	}

	ws := NewWorkspace()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	ws.Open("Requirement Definitions.sysml", content, 1)
	diags := ws.Diagnostics("Requirement Definitions.sysml")

	var errs []string
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			errs = append(errs, d.Message)
			t.Logf("ERROR: %s", d.Message)
		}
	}

	if len(errs) > 0 {
		t.Fatalf("Expected 0 errors, got %d", len(errs))
	}
}
