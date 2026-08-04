package model

import (
	"testing"
)

func TestRequirementParameterBinding(t *testing.T) {
	ws := NewWorkspace()
	
	src := `package test {
		import ScalarValues::*;
		
		part def Vehicle {
			attribute mass : Real;
		}
		
		requirement def MassLimit {
			subject vehicle : Vehicle;
			
			require constraint {
				vehicle.mass < 1000.0
			}
		}
		
		requirement checkVehicle : MassLimit {
			subject testVehicle = Vehicle();
		}
	}`
	
	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")
	
	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}
	
	// Should have 0 diagnostics - vehicle should resolve in requirement constraint
	if len(diags) > 0 {
		t.Errorf("Expected 0 diagnostics, got %d", len(diags))
	}
}
