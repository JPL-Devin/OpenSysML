package passes

import (
	"strings"
	"testing"
)

func TestIdentityDuplicateDeclaredIdsInOneScope(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
	part def B {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 2 {
		t.Fatalf("got %d duplicate-id diagnostics, want 2: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "P::A") || !strings.Contains(d.Message, "P::B") {
			t.Fatalf("diagnostic must name both elements: %q", d.Message)
		}
	}
}

func TestIdentitySameIdUnderDifferentProjectsIsLegal(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
	package Q {
		@IdentityMetadata::ProjectRef { projectId = "proj-2"; }
		part def B {
			@IdentityMetadata::ElementId { id = "same-id"; }
		}
	}
}
`
	w8dWantLines(t, src, "identity-duplicate-id")
}

func TestIdentitySameProjectOnTwoBranchesIsOneScope(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
	package Q {
		@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "dev"; }
		part def B {
			@IdentityMetadata::ElementId { id = "same-id"; }
		}
	}
}
`
	w8dWantLines(t, src, "identity-duplicate-id", 4, 9)
}

func TestIdentityDeclaredIdCollidingWithADerivedIdErrorsOnBoth(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	part def Y {
		@IdentityMetadata::ElementId { id = "P__X"; }
	}
}
`
	w8dWantLines(t, src, "identity-duplicate-id", 3, 5)
}

func TestIdentityDeclaredIdEndingInOmCollidesWithAMembershipId(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	part def Y {
		@IdentityMetadata::ElementId { id = "P__X_om"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "owning-membership") ||
			!strings.Contains(d.Message, "P::X") || !strings.Contains(d.Message, "P::Y") {
			t.Fatalf("diagnostic must name both elements and the membership space: %q", d.Message)
		}
	}
}

func TestIdentityDeclaredIdEmbeddingPCollidesWithAnExpressionNodeId(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	part def Y {
		@IdentityMetadata::ElementId { id = "P__X_p3"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "expression-node") ||
			!strings.Contains(d.Message, "P::X") || !strings.Contains(d.Message, "P::Y") {
			t.Fatalf("diagnostic must name both elements and the expression space: %q", d.Message)
		}
	}
}

func TestIdentityIdShapeErrorNamesTheOffendingByte(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "bad id"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-id-shape")
	if len(diags) != 1 {
		t.Fatalf("got %d shape diagnostics, want 1: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "0x20") || !strings.Contains(diags[0].Message, "offset 3") {
		t.Fatalf("diagnostic must name the offending byte and offset: %q", diags[0].Message)
	}
}

func TestIdentityElementIdWithoutEnclosingProjectRef(t *testing.T) {
	src := `package P {
	part def A {
		@IdentityMetadata::ElementId { id = "some-id"; }
	}
}
`
	w8dWantLines(t, src, "identity-unscoped-id", 3)
}

func TestIdentityNestedProjectRefScopesArePermitted(t *testing.T) {
	src := `package Outer {
	@IdentityMetadata::ProjectRef { projectId = "outer-proj"; }
	part def A {
		@IdentityMetadata::ElementId { id = "id-a"; }
	}
	package Inner {
		@IdentityMetadata::ProjectRef { projectId = "inner-proj"; }
		part def B {
			@IdentityMetadata::ElementId { id = "id-b"; }
		}
	}
}
`
	for _, code := range []string{"identity-duplicate-id", "identity-id-shape", "identity-unscoped-id"} {
		w8dWantLines(t, src, code)
	}
}

func TestIdentityLegalAnnotationsStaySilent(t *testing.T) {
	src := `package Vehicles {
	@IdentityMetadata::ProjectRef { projectId = "b3f9c2e8"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0-1c22-4d9e-9c3b-000000000001"; }
	}
	part def Wheel;
}
`
	for _, d := range w8dDiags(t, src) {
		if strings.HasPrefix(d.Code, "identity-") {
			t.Fatalf("unexpected identity diagnostic: %+v", d)
		}
	}
}
