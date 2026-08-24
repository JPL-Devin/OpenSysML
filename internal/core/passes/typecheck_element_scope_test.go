package passes

import (
	"strings"
	"testing"
)

func TestTransitionGuardIsElementScoped(t *testing.T) {
	src := `package P {
	part broken : Missing;
	state def S {
		state a;
		state b;
		transition a to b if "test";
	}
}`
	diags := diagsIn(t, "a.sysml", src, "type")
	if len(diags) != 1 ||
		diags[0].Message != "transition guard must be Boolean, found String" ||
		diags[0].Span.Offset != strings.Index(src, `"test"`) {
		t.Fatalf("got %v, want only the independent transition guard diagnostic", diags)
	}
}

func TestTransitionGuardUnresolvedDoesNotCascade(t *testing.T) {
	src := `package P {
	state def S {
		state a;
		state b;
		transition a to b if missing;
	}
}`
	if diags := diagsIn(t, "a.sysml", src, "type"); len(diags) != 0 {
		t.Fatalf("got %v, want no type cascade for the unresolved guard", diags)
	}
}

func TestKerMLSubsettingMetaclassIsElementScoped(t *testing.T) {
	src := `package P {
	classifier non;
	feature aa subsets non;
	feature broken subsets missing;
}`
	diags := diagsIn(t, "a.kerml", src, "type")
	if len(diags) != 1 ||
		diags[0].Message != "subsets target must be a feature, found kermlType" ||
		diags[0].Span.Offset != strings.Index(src, "non;\n\tfeature broken") {
		t.Fatalf("got %v, want only the independent subsetting diagnostic", diags)
	}
}

func TestKerMLSubsettingMetaclassUnresolvedDoesNotCascade(t *testing.T) {
	src := `package P {
	feature aa subsets missing;
}`
	if diags := diagsIn(t, "a.kerml", src, "type"); len(diags) != 0 {
		t.Fatalf("got %v, want no type cascade for the unresolved subset target", diags)
	}
}
