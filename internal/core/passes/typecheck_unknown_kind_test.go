package passes

import (
	"strings"
	"testing"
)

// A KerML declaration such as `classifier` has no usage-kind counterpart, so no
// kind mismatch can be asserted against it as a target.
func TestTypeCheckUnknownTargetKindNoMismatch(t *testing.T) {
	diags := typeDiags(t, "classifier T; part def Car { part p : T; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

// That a definition may not subset or redefine a feature is a property of the
// declaration, so an unclassified target does not excuse it.
func TestTypeCheckDefinitionSubsetsUnknownTarget(t *testing.T) {
	for _, src := range []string{
		"classifier T; part def Car subsets T;",
		"classifier T; part def Car redefines T;",
	} {
		diags := typeDiags(t, src)
		if len(diags) != 1 {
			t.Fatalf("%s: expected one type diagnostic, got %v", src, diags)
		}
		if !strings.Contains(diags[0].Message, "a definition may not") {
			t.Errorf("%s: got %q", src, diags[0].Message)
		}
	}
}
