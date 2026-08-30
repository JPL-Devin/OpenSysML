package view

import "testing"

// A rendering names an element the way the notation declares it: several
// spellings share one kind, and a KerML classifier takes no `def`.
func TestRenderingsNameElementsAsWritten(t *testing.T) {
	_, idx := loadFixture(t, "notation.sysml")
	want := map[string]string{
		"Notation::M":           "metaclass",
		"Notation::T":           "datatype",
		"Notation::f":           "feature",
		"Notation::C":           "class",
		"Notation::BD":          "behavior def",
		"Notation::Wheel":       "part def",
		"Notation::my wheel":    "part",
		"Notation::Run::moving": "assert constraint",
		"Notation::Run::drive":  "perform action",
	}
	for fqn, kind := range want {
		if got := declKind(lookup(t, idx, fqn)); got != kind {
			t.Errorf("%s renders as %q, want %q", fqn, got, kind)
		}
	}
}
