package model

import (
	"os"
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

func TestRequirementDefinitionsFile(t *testing.T) {
	ws := NewWorkspace()
	
	path := "../../../examples/sysml-v2-training/32. Requirements/Requirement Definitions.sysml"
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
