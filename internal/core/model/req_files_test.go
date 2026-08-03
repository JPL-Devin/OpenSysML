package model

import (
	"os"
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

func TestRequirementFilesIntegration(t *testing.T) {
	ws := NewWorkspace()

	// Load in dependency order
	files := []string{
		"../../../examples/sysml-v2-training/32. Requirements/Requirement Definitions.sysml",
		"../../../examples/sysml-v2-training/32. Requirements/Requirement Usages.sysml",
		"../../../examples/sysml-v2-training/32. Requirements/Requirement Groups.sysml",
	}

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", path, err)
		}
		ws.Open(path, content, 1)
		t.Logf("Loaded: %s", path)
	}

	// Check Groups file errors
	diags := ws.Diagnostics("../../../examples/sysml-v2-training/32. Requirements/Requirement Groups.sysml")
	
	var errs []string
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			errs = append(errs, d.Message)
			t.Logf("ERROR: %s", d.Message)
		}
	}

	t.Logf("Total errors in Groups file: %d", len(errs))
}
