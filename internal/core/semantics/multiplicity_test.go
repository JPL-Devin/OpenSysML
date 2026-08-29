package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

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

// A feature that declares no multiplicity holds exactly one value, so the
// effective multiplicity of a bare usage is the assumed 1..1.
func TestEffectiveMultiplicityAssumesOne(t *testing.T) {
	m, root := buildModel(t, "part def C { part a; part b [0..*]; }")
	c := sym(t, root, "C")

	a, _ := c.Scope.LookupLocal("a")
	if r := m.EffectiveMultiplicityOf(a); r != AssumedRange() {
		t.Errorf("EffectiveMultiplicityOf(a) = %+v, want %+v", r, AssumedRange())
	}
	if m.EffectiveMultiplicityOf(a).CountViolation(2) == "" {
		t.Error("two values conform to an undeclared multiplicity, want a violation")
	}

	// A declared multiplicity is the effective one, assumed nowhere.
	b, _ := c.Scope.LookupLocal("b")
	declared, ok := m.MultiplicityOf(b)
	if !ok {
		t.Fatal("MultiplicityOf(b) not ok")
	}
	if r := m.EffectiveMultiplicityOf(b); r != declared {
		t.Errorf("EffectiveMultiplicityOf(b) = %+v, want the declared %+v", r, declared)
	}
}

// CountViolation is the shared wording for a count against a multiplicity: an
// unbounded or unknown bound admits any count, either side of a stated one does
// not.
func TestRangeCountViolation(t *testing.T) {
	known := func(v int64) Bound { return Bound{Value: v, Known: true} }
	for _, tc := range []struct {
		name  string
		r     Range
		count int64
		want  string
	}{
		{"exact conforms", Range{known(3), known(3)}, 3, ""},
		{"too few", Range{known(3), known(3)}, 1, "1 value(s) bound to a feature with multiplicity lower bound 3"},
		{"too many", Range{known(3), known(3)}, 4, "4 value(s) bound to a feature with multiplicity upper bound 3"},
		{"none against a lower bound", Range{known(1), known(3)}, 0, "0 value(s) bound to a feature with multiplicity lower bound 1"},
		{"unbounded upper admits any count", Range{known(0), Bound{Infinite: true, Known: true}}, 7, ""},
		{"one or more admits one", Range{known(1), Bound{Infinite: true, Known: true}}, 1, ""},
		{"one or more rejects none", Range{known(1), Bound{Infinite: true, Known: true}}, 0, "0 value(s) bound to a feature with multiplicity lower bound 1"},
		{"unknown bounds admit any count", Range{}, 5, ""},
	} {
		if got := tc.r.CountViolation(tc.count); got != tc.want {
			t.Errorf("%s: CountViolation(%d) = %q, want %q", tc.name, tc.count, got, tc.want)
		}
	}
}

// A bound that is not a literal is not evaluable, so the ordering check reports
// ok=false and a caller skips it rather than diagnosing an unknown bound.
func TestMultiplicityWithANonEvaluableBound(t *testing.T) {
	m, root := buildModel(t, "part def C { attribute n; part a [n..5]; }")
	c := sym(t, root, "C")
	a, _ := c.Scope.LookupLocal("a")
	r, ok := m.MultiplicityOf(a)
	if !ok {
		t.Fatal("MultiplicityOf(a) not ok: the usage declares a multiplicity")
	}
	if r.Lower.Known {
		t.Errorf("lower = %+v, want unknown", r.Lower)
	}
	if _, evalOK := r.LowerLeUpper(); evalOK {
		t.Error("LowerLeUpper is evaluable with an unknown lower bound")
	}
	if msg := r.CountViolation(0); msg != "" {
		t.Errorf("CountViolation against an unknown lower bound = %q, want none", msg)
	}
}

// An infinite lower bound only orders against an infinite upper one, so `[*..2]`
// is a stated, evaluable, invalid range rather than an unknown one.
func TestMultiplicityInfiniteLowerWithFiniteUpper(t *testing.T) {
	m, root := buildModel(t, "part def C { part a [*..2]; }")
	c := sym(t, root, "C")
	a, _ := c.Scope.LookupLocal("a")
	r, ok := m.MultiplicityOf(a)
	if !ok {
		t.Fatal("MultiplicityOf(a) not ok")
	}
	valid, evalOK := r.LowerLeUpper()
	if !evalOK {
		t.Fatal("LowerLeUpper not evaluable: both bounds are stated")
	}
	if valid {
		t.Errorf("[*..2] = %+v reported a valid range", r)
	}
}

// Multiplicity is a property of a usage, so a definition and a nil symbol have
// none and take the assumed range.
func TestMultiplicityOfANonUsage(t *testing.T) {
	m, root := buildModel(t, "part def C { part a [2]; }")
	c := sym(t, root, "C")
	for name, s := range map[string]*symbols.Symbol{"definition": c, "nil": nil} {
		if _, ok := m.MultiplicityOf(s); ok {
			t.Errorf("%s: MultiplicityOf reported a multiplicity", name)
		}
		if r := m.EffectiveMultiplicityOf(s); r != AssumedRange() {
			t.Errorf("%s: EffectiveMultiplicityOf = %+v, want the assumed range", name, r)
		}
	}
}
