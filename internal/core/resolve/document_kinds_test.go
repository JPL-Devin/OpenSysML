package resolve

import "testing"

func TestResolveConnectorEndsClean(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"part def Sys { part a; part b; connection c connect a to b; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveFlowEndsAndPayloadClean(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"part def Sys { item Fuel; part a; part b; flow f of Fuel from a to b; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveUnresolvedConnectorEnd(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"part def Sys { part a; connection c connect a to missing; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved-name diagnostic for the 'missing' end")
	}
}
