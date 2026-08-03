package model

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

func TestDocumentNameVsPackageName(t *testing.T) {
	ws := NewWorkspace()

	// Load with path-like document name (like training test does)
	usagesSrc := `package 'Requirement Usages' {
		requirement def Usage1;
	}`
	ws.Open("folder/Requirement Usages.sysml", []byte(usagesSrc), 1)
	
	groupsSrc := `package 'Requirement Groups' {
		private import 'Requirement Usages'::*;
	}`
	ws.Open("folder/Requirement Groups.sysml", []byte(groupsSrc), 1)
	
	// Check if import resolved
	diags := ws.Diagnostics("folder/Requirement Groups.sysml")
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			t.Logf("ERROR: %s", d.Message)
		}
	}
	
	// Check if re-export worked
	usage1 := ws.index.LookupQualified("Requirement Groups::Usage1")
	t.Logf("'Requirement Groups::Usage1' symbols: %d", len(usage1))
	
	if len(usage1) == 0 {
		t.Error("Import/re-export failed with path-like document names")
	}
}
