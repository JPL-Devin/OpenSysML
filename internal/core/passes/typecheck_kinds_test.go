package passes

import "testing"

func TestTypeCheckNewKindSpecializeSameKindOK(t *testing.T) {
	diags := typeDiags(t, "item def Base; item def Sub specializes Base;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckNewKindSpecializeCrossKindError(t *testing.T) {
	diags := typeDiags(t, "part def P; item def I specializes P;")
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
}

func TestTypeCheckNewKindTypingWantsMatchingDef(t *testing.T) {
	// A port usage typed by a part def is a cross-kind error.
	diags := typeDiags(t, "part def Q; part def Sys { port p : Q; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

func TestTypeCheckNewKindTypingMatchingDefOK(t *testing.T) {
	diags := typeDiags(t, "port def Q; part def Sys { port p : Q; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}
