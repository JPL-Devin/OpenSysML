package repl

import (
	"strings"
	"testing"
)

// The session index carries the standard library, so a measurement unit named
// by a quantity expression resolves at the prompt.
func TestSessionIndexResolvesLibraryUnit(t *testing.T) {
	s := NewSession()
	s.Submit("package P { import SI::*; attribute speed = 1.5 [m/s]; }")
	out, _, err := s.runMeta("%eval P::speed")
	if err != nil {
		t.Fatalf("%%eval P::speed: %v", err)
	}
	if got := strings.Join(out, "\n"); !strings.Contains(got, "m/s") {
		t.Fatalf("want the unit in the value, got %q", got)
	}
}

// Redeclaring without an import drops the names it brought in: the index is
// built per submission, so nothing an earlier import re-exported survives.
func TestSessionIndexDropsRemovedImport(t *testing.T) {
	s := NewSession()
	s.Submit("package P { import SI::*; }")
	if _, _, err := s.lookupSymbol("P::metre"); err != nil {
		t.Fatalf("imported name should resolve: %v", err)
	}
	s.Submit("package P { }")
	if _, fqn, err := s.lookupSymbol("P::metre"); err == nil {
		t.Fatalf("removed import still resolves as %q", fqn)
	}
}
