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

// A single unbounded bound is 0..*, not *..*: the lower bound follows the upper
// one only when the upper one is bounded (KerML 1.0 §8.2.5.11).
func TestMultiplicitySingleBoundStar(t *testing.T) {
	m, root := buildModel(t, "part def C { part parts [*]; part exact [*..*]; }")
	c := sym(t, root, "C")

	parts, _ := c.Scope.LookupLocal("parts")
	r, ok := m.MultiplicityOf(parts)
	if !ok {
		t.Fatalf("MultiplicityOf(parts) not ok")
	}
	if !r.Lower.Known || r.Lower.Infinite || r.Lower.Value != 0 {
		t.Fatalf("[*] lower = %+v, want 0", r.Lower)
	}
	if !r.Upper.Known || !r.Upper.Infinite {
		t.Fatalf("[*] upper = %+v, want infinite", r.Upper)
	}
	if valid, evalOK := r.LowerLeUpper(); !evalOK || !valid {
		t.Fatalf("[*] LowerLeUpper = %v, %v; want true, true", valid, evalOK)
	}

	exact, _ := c.Scope.LookupLocal("exact")
	r, ok = m.MultiplicityOf(exact)
	if !ok {
		t.Fatalf("MultiplicityOf(exact) not ok")
	}
	if !r.Lower.Infinite || !r.Upper.Infinite {
		t.Fatalf("[*..*] = %+v, want both bounds infinite", r)
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
