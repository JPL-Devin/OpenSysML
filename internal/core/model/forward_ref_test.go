package model

import (
	"testing"
)

func TestForwardReferenceWildcardExpansion(t *testing.T) {
	ws := NewWorkspace()

	// Load Groups FIRST (imports 'Requirement Usages'::* which doesn't exist yet)
	groupsSrc := `package 'Requirement Groups' {
		private import 'Requirement Usages'::*;
		
		requirement def Group1 {
			// Should be able to use members from 'Requirement Usages' after it loads
		}
	}`
	
	ws.Open("Groups.sysml", []byte(groupsSrc), 1)
	
	// Check wildcard metadata stored
	syms := ws.index.LookupQualified("Requirement Groups")
	t.Logf("Found %d symbols for 'Requirement Groups'", len(syms))
	if len(syms) == 0 {
		t.Fatal("Requirement Groups package not indexed")
	}
	
	// Load Usages SECOND (defines the package Groups imports)
	usagesSrc := `package 'Requirement Usages' {
		requirement def Usage1;
		requirement def Usage2;
	}`
	
	ws.Open("Usages.sysml", []byte(usagesSrc), 1)
	
	// Check that Usages package indexed
	usageSyms := ws.index.LookupQualified("Requirement Usages")
	t.Logf("Found %d symbols for 'Requirement Usages'", len(usageSyms))
	if len(usageSyms) == 0 {
		t.Fatal("Requirement Usages package not indexed")
	}
	
	// Check if Usage1/Usage2 are re-exported under Groups
	usage1InGroups := ws.index.LookupQualified("Requirement Groups::Usage1")
	t.Logf("'Requirement Groups::Usage1' symbols: %d", len(usage1InGroups))
	
	usage2InGroups := ws.index.LookupQualified("Requirement Groups::Usage2")
	t.Logf("'Requirement Groups::Usage2' symbols: %d", len(usage2InGroups))
	
	if len(usage1InGroups) == 0 {
		t.Error("Usage1 not re-exported to Requirement Groups (wildcard expansion failed)")
	}
	if len(usage2InGroups) == 0 {
		t.Error("Usage2 not re-exported to Requirement Groups (wildcard expansion failed)")
	}
}
