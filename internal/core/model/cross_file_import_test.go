package model

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

func TestCrossFileWildcardImport(t *testing.T) {
	ws := NewWorkspace()
	
	// File 1: Defines MassedThing
	file1 := `package MassRollup1 {
		part def MassedThing {
			attribute simpleMass :> ISQ::mass;
		}
		part simpleThing : MassedThing {}
	}`
	
	// File 2: Imports and uses MassedThing
	file2 := `package TestPkg {
		private import MassRollup1::*;
		
		part def CarPart :> MassedThing {}
		part test :> simpleThing {}
	}`
	
	ws.Open("MassRollup1.sysml", []byte(file1), 1)
	ws.Open("Test.sysml", []byte(file2), 1)
	
	diags := ws.Diagnostics("Test.sysml")
	
	var errs []string
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			errs = append(errs, d.Message)
			t.Logf("ERROR: %s", d.Message)
		}
	}
	
	if len(errs) > 0 {
		t.Fatalf("Expected cross-file wildcard import to work, got %d errors", len(errs))
	}
	t.Log("✓ Cross-file wildcard import resolved MassedThing")
}
