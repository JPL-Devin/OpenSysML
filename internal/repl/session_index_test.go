package repl

import "testing"

// A submission that no longer states a wildcard import must not go on resolving
// the names that import surfaced. The session keeps one index across
// submissions, so this is what makes reuse safe rather than stale.
func TestSubmitDropsAReplacedWildcardImport(t *testing.T) {
	s := NewSession()
	s.Submit("package Lib { part def Widget; }")
	s.Submit("package P { public import Lib::*; }")

	if _, _, err := s.lookupSymbol("P::Widget"); err != nil {
		t.Fatalf("P::Widget after importing Lib::*: %v", err)
	}
	indexed := s.symbolIndex()

	s.Submit("package P { }")
	if _, _, err := s.lookupSymbol("P::Widget"); err == nil {
		t.Error("P::Widget still resolves after the submission dropping `import Lib::*`")
	}
	if _, _, err := s.lookupSymbol("Lib::Widget"); err != nil {
		t.Errorf("Lib::Widget after the import was dropped: %v", err)
	}
	if s.symbolIndex() != indexed {
		t.Error("the session rebuilt its index instead of updating the one it had")
	}
}

// Re-stating the import surfaces the names again.
func TestSubmitRestoresAReinstatedWildcardImport(t *testing.T) {
	s := NewSession()
	s.Submit("package Lib { part def Widget; }")
	s.Submit("package P { public import Lib::*; }")
	s.Submit("package P { }")
	s.Submit("package P { public import Lib::*; }")

	if _, _, err := s.lookupSymbol("P::Widget"); err != nil {
		t.Errorf("P::Widget after re-stating the import: %v", err)
	}
}
