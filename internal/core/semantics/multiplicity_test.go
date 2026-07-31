package semantics

import "testing"

func TestMultiplicitySingleBound(t *testing.T) {
	m, root := buildModel(t, "part def C { part wheels [4]; }")
	c := sym(t, root, "C")
	wheels, ok := c.Scope.LookupLocal("wheels")
	if !ok {
		t.Fatalf("wheels not found")
	}
	r, ok := m.MultiplicityOf(wheels)
	if !ok {
		t.Fatalf("MultiplicityOf(wheels) not ok")
	}
	if !r.Lower.Known || r.Lower.Value != 4 || !r.Upper.Known || r.Upper.Value != 4 {
		t.Fatalf("single bound [4] = %+v, want lower=upper=4", r)
	}
}

func TestMultiplicityRangeStar(t *testing.T) {
	m, root := buildModel(t, "part def C { part parts [0..*]; }")
	c := sym(t, root, "C")
	parts, _ := c.Scope.LookupLocal("parts")
	r, ok := m.MultiplicityOf(parts)
	if !ok {
		t.Fatalf("MultiplicityOf(parts) not ok")
	}
	if !r.Lower.Known || r.Lower.Value != 0 {
		t.Fatalf("lower = %+v, want 0", r.Lower)
	}
	if !r.Upper.Known || !r.Upper.Infinite {
		t.Fatalf("upper = %+v, want infinite", r.Upper)
	}
}

func TestMultiplicityLowerLeUpper(t *testing.T) {
	m, root := buildModel(t, "part def C { part a [2..5]; part b [5..2]; part c [1..*]; }")
	c := sym(t, root, "C")

	for _, tc := range []struct {
		name      string
		wantValid bool
	}{
		{"a", true},
		{"b", false},
		{"c", true},
	} {
		u, _ := c.Scope.LookupLocal(tc.name)
		r, ok := m.MultiplicityOf(u)
		if !ok {
			t.Fatalf("%s: MultiplicityOf not ok", tc.name)
		}
		valid, evalOK := r.LowerLeUpper()
		if !evalOK {
			t.Fatalf("%s: LowerLeUpper not evaluable", tc.name)
		}
		if valid != tc.wantValid {
			t.Fatalf("%s: valid = %v, want %v", tc.name, valid, tc.wantValid)
		}
	}
}

func TestMultiplicityNoneWhenAbsent(t *testing.T) {
	m, root := buildModel(t, "part def C { part a; }")
	c := sym(t, root, "C")
	a, _ := c.Scope.LookupLocal("a")
	if _, ok := m.MultiplicityOf(a); ok {
		t.Fatalf("expected no multiplicity for bare usage")
	}
}
