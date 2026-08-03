package model

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"os"
)

func TestMassRollupExample(t *testing.T) {
	ws := NewWorkspace()
	
	// Load both files
	mr1, _ := os.ReadFile("../../../examples/sysml-v2-training/29. Expressions/MassRollup1.sysml")
	ex1, _ := os.ReadFile("../../../examples/sysml-v2-training/29. Expressions/Car Mass Rollup Example 1.sysml")
	
	ws.Open("29. Expressions/MassRollup1.sysml", mr1, 1)
	ws.Open("29. Expressions/Car Mass Rollup Example 1.sysml", ex1, 1)
	
	diags := ws.Diagnostics("29. Expressions/Car Mass Rollup Example 1.sysml")
	
	t.Logf("Diagnostics: %d", len(diags))
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			t.Logf("ERROR: %s", d.Message)
		}
	}
}
