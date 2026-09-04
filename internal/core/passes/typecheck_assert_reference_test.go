package passes

import (
	"strings"
	"testing"
)

// The reference form of an assert constraint usage must reference a constraint
// usage (SysML v2 §7.19, validateAssertConstraintUsageReference).
func TestAssertReferenceToConstraintIsSilent(t *testing.T) {
	src := `constraint def CD; requirement def RD;
part def Base { constraint inherited : CD; }
part def Derived :> Base { constraint c : CD; requirement r : RD; }
part v : Derived;
part ctx {
	constraint local : CD;
	assert local;
	assert not local;
	assert v.c;
	assert not v.c;
	assert v.inherited;
	assert v.r;
	assert v.c { true }
	assert constraint declared : CD;
	assert constraint { true }
}`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

func TestAssertReferenceToNonConstraintRejected(t *testing.T) {
	tests := []struct {
		name, target, found string
	}{
		{"part usage", "assert p;", "partUsage"},
		{"negated part usage", "assert not p;", "partUsage"},
		{"attribute usage", "assert a;", "attributeUsage"},
		{"with body", "assert p { true }", "partUsage"},
		{"chained part usage", "assert v.p;", "partUsage"},
		{"negated chained attribute", "assert not v.a;", "attributeUsage"},
		{"chained inherited part", "assert v.bp;", "partUsage"},
		{"constraint definition", "assert CD;", "constraintDef"},
		{"part definition", "assert PD;", "partDef"},
		{"package", "assert Q;", "package"},
	}
	prefix := `constraint def CD; part def PD; package Q;
part def Base { part bp; }
part def Derived :> Base { part p; attribute a; }
part v : Derived; part p; attribute a;
part ctx { `
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := typeDiags(t, prefix+tc.target+" }")
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			want := "assert target must be a constraint usage, found " + tc.found
			if diags[0].Message != want {
				t.Errorf("got %q, want %q", diags[0].Message, want)
			}
		})
	}
}

// An unresolved assert target is the name-resolution tier's finding alone.
func TestAssertReferenceUnresolvedIsNotAKindError(t *testing.T) {
	src := "part def Derived; part v : Derived; part ctx { assert missing; assert v.missing; }"
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

// `satisfy` shares the referent-kind rule, so a chained target is checked too.
func TestSatisfyChainedNonRequirementRejected(t *testing.T) {
	src := "constraint def CD; requirement def RD; part def D { constraint c : CD; requirement r : RD; }" +
		" part v : D; part ctx { satisfy v.r; satisfy v.c; }"
	diags := typeDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "satisfy target must be a requirement usage, found constraintUsage") {
		t.Errorf("got %q", diags[0].Message)
	}
}
