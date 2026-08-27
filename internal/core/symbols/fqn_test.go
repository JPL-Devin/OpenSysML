package symbols

import "testing"

// HasFQN answers what FQNOf would build, for the name a symbol has and for the
// prefixes, suffixes and near-misses of it that it does not.
func TestHasFQNMatchesFQNOf(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { package Q { part def Vehicle { attribute mass; } } }")

	for _, fqn := range []string{"P", "P::Q", "P::Q::Vehicle", "P::Q::Vehicle::mass"} {
		syms := idx.LookupQualified(fqn)
		if len(syms) != 1 {
			t.Fatalf("LookupQualified(%s) len = %d, want 1", fqn, len(syms))
		}
		sym := syms[0]
		if got := FQNOf(sym); got != fqn {
			t.Fatalf("FQNOf(%s) = %q", fqn, got)
		}
		if !HasFQN(sym, fqn) {
			t.Errorf("HasFQN(%s, %q) = false, want true", fqn, fqn)
		}
		for _, other := range []string{"", "Q", "Vehicle", "mass", "P::Q::Vehicle::mass::x",
			"P::Q::Vehicle", "X::" + fqn, fqn + "::x", fqn + "x"} {
			if want := other == fqn; HasFQN(sym, other) != want {
				t.Errorf("HasFQN(%s, %q) = %v, want %v", fqn, other, !want, want)
			}
		}
	}
}

func TestHasFQNNilSymbol(t *testing.T) {
	if !HasFQN(nil, "") {
		t.Error(`HasFQN(nil, "") = false, want true`)
	}
	if HasFQN(nil, "P") {
		t.Error(`HasFQN(nil, "P") = true, want false`)
	}
}
