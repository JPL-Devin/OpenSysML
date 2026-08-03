package model

import (
	"testing"
)

func TestQuotedPackageImport(t *testing.T) {
	ws := NewWorkspace()

	// Load dependency first
	dep := `package 'My Package' {
		part def Engine;
	}`
	ws.Open("dep.sysml", []byte(dep), 1)

	// Import with quoted name
	main := `package Test {
		private import 'My Package'::*;
		
		part myEngine : Engine;
	}`
	ws.Open("main.sysml", []byte(main), 1)
	
	// Check if Engine resolves
	diags := ws.Diagnostics("main.sysml")
	for _, d := range diags {
		t.Logf("%s: %s", d.Severity, d.Message)
	}
	
	// Should have no unresolved errors
	var unresolvedCount int
	for _, d := range diags {
		if d.Message == "unresolved reference: Engine" {
			unresolvedCount++
		}
	}
	
	if unresolvedCount > 0 {
		t.Errorf("Engine not resolved - quoted package import failed")
	}
}
