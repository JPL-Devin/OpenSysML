package export_test

import (
	"strings"
	"testing"
)

// The operator a feature value is written with decides whether a redefinition
// may override it, so `default` and `:=` must survive the trip through RDF.
func TestFeatureValueOperatorsSurviveRDF(t *testing.T) {
	turtle := roundTripsExactly(t, `package P {
	private import ScalarValues::Integer;
	part def V {
		attribute bound : Integer = 1;
		attribute overridable : Integer default = 2;
		attribute initial : Integer := 3;
		attribute both : Integer default := 4;
	}
	part def W;
	part w0 : W;
	requirement req {
		subject w : W default = w0;
	}
}
`)
	flagsOf := func(element string) (isDefault, isInitial bool) {
		_, rest, found := strings.Cut(string(turtle), "\n"+element+"\n")
		if !found {
			t.Fatalf("graph lacks %s:\n%s", element, turtle)
		}
		block, _, _ := strings.Cut(rest, " .\n")
		return strings.Contains(block, `sysml:isDefault "true"^^xsd:boolean`),
			strings.Contains(block, `sysml:isInitial "true"^^xsd:boolean`)
	}
	for element, want := range map[string][2]bool{
		"elmt:P__V__bound":       {false, false},
		"elmt:P__V__overridable": {true, false},
		"elmt:P__V__initial":     {false, true},
		"elmt:P__V__both":        {true, true},
		"elmt:P__req__w":         {true, false},
	} {
		isDefault, isInitial := flagsOf(element)
		if isDefault != want[0] || isInitial != want[1] {
			t.Errorf("%s: isDefault=%v isInitial=%v, want %v %v", element, isDefault, isInitial, want[0], want[1])
		}
	}
}

// A requirement's `assume`/`require constraint` declares a constraint usage of
// its own, whose name, specializations, multiplicity and value are as much a
// part of the model as those of any other usage.
func TestOwnedConstraintDeclarationsSurviveRDF(t *testing.T) {
	roundTripsExactly(t, `package P {
	constraint def C;
	constraint c0 : C;
	requirement def R {
		assume constraint c1 : C[1] = c0;
		require constraint c2 : C default = c0;
		require constraint c3 subsets c0 := c0;
		assume constraint c4 : C subsets c0[1] default := c0 {
			true;
		}
		require constraint c5;
		assume constraint c6 {
		}
		require c0[1] subsets c0;
	}
}
`)
}
