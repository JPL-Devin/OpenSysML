package symbols

import "testing"

func TestScopeMemberNamesInOrder(t *testing.T) {
	s := NewScope(nil, nil)
	s.Define("Beta", &Symbol{Name: "Beta", Kind: SymbolPackage})
	s.Define("Alpha", &Symbol{Name: "Alpha", Kind: SymbolNamespace})
	// Duplicate key must not add a second memberOrder entry.
	s.Define("Beta", &Symbol{Name: "Beta", Kind: SymbolNamespace})

	names := s.MemberNames()
	if len(names) != 2 || names[0] != "Beta" || names[1] != "Alpha" {
		t.Fatalf("MemberNames = %v, want [Beta Alpha]", names)
	}
}
