package model

import (
	"os"
	"path/filepath"
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

func TestRequirementGroupsForwardRef(t *testing.T) {
	ws := NewWorkspace()
	
	trainingDir := filepath.Join("..", "..", "..", "examples", "sysml-v2-training", "32. Requirements")
	
	// Load in alphabetical order (Groups before Usages)
	files := []string{
		"Requirement Definitions.sysml",
		"Requirement Groups.sysml",
		"Requirement Usages.sysml",
	}
	
	for _, name := range files {
		path := filepath.Join(trainingDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", name, err)
		}
		
		ws.Open(name, content, 1)
		t.Logf("Loaded %s", name)
	}
	
	// Check if Groups can see Usages members
	fullVehicleInGroups := ws.index.LookupQualified("Requirement Groups::fullVehicleMassLimit")
	t.Logf("'Requirement Groups::fullVehicleMassLimit' symbols: %d", len(fullVehicleInGroups))
	
	emptyVehicleInGroups := ws.index.LookupQualified("Requirement Groups::emptyVehicleMassLimit")
	t.Logf("'Requirement Groups::emptyVehicleMassLimit' symbols: %d", len(emptyVehicleInGroups))
	
	// Check diagnostics
	diags := ws.Diagnostics("Requirement Groups.sysml")
	var errs []string
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			errs = append(errs, d.Message)
			t.Logf("ERROR: %s", d.Message)
		}
	}
	
	if len(fullVehicleInGroups) == 0 {
		t.Error("fullVehicleMassLimit not re-exported (forward ref failed)")
	}
	if len(emptyVehicleInGroups) == 0 {
		t.Error("emptyVehicleMassLimit not re-exported (forward ref failed)")
	}
	
	t.Logf("Total errors: %d", len(errs))
}
