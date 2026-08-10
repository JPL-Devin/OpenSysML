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

// An objective is the ownedObjectiveRequirement of an ObjectiveMembership and so
// is a RequirementUsage (SysML v2 §8.3.22.4): a requirement definition and its
// specializations type it, structural definitions do not.
func TestTypeCheckObjectiveTypedByRequirementDefOK(t *testing.T) {
	diags := typeDiags(t, "requirement def MaximizeObjective; analysis def A { objective : MaximizeObjective; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckObjectiveTypedByConcernDefOK(t *testing.T) {
	diags := typeDiags(t, "concern def C; analysis def A { objective o : C; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckObjectiveTypedByPartDefError(t *testing.T) {
	diags := typeDiags(t, "part def Vehicle; analysis def A { objective o : Vehicle; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

func TestTypeCheckObjectiveTypedByActionDefError(t *testing.T) {
	diags := typeDiags(t, "action def Move; analysis def A { objective o : Move; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

// A subject is an unconstrained Usage, so a structural definition still types it
// while a requirement definition does not.
func TestTypeCheckSubjectTypedByPartDefOK(t *testing.T) {
	diags := typeDiags(t, "part def Vehicle; analysis def A { subject v : Vehicle; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckSubjectTypedByRequirementDefError(t *testing.T) {
	diags := typeDiags(t, "requirement def R2; analysis def A { subject s : R2; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}
