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

func TestScopeForEachMemberMatchesAllMembers(t *testing.T) {
	s := NewScope(nil, nil)
	first := &Symbol{Name: "first"}
	second := &Symbol{Name: "second"}
	anonymous := &Symbol{}
	s.Define("first", first)
	s.Define("second", second)
	s.DefineAnonymous(anonymous)

	want := s.AllMembers()
	var got []*Symbol
	s.ForEachMember(func(sym *Symbol) bool {
		got = append(got, sym)
		return true
	})
	if len(got) != len(want) {
		t.Fatalf("ForEachMember visited %d symbols, AllMembers returned %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEachMember[%d] = %p, AllMembers[%d] = %p", i, got[i], i, want[i])
		}
	}
}

func TestScopeForEachMemberStopsWhenYieldReturnsFalse(t *testing.T) {
	s := NewScope(nil, nil)
	s.Define("first", &Symbol{Name: "first"})
	s.Define("second", &Symbol{Name: "second"})
	s.DefineAnonymous(&Symbol{})

	count := 0
	s.ForEachMember(func(*Symbol) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("ForEachMember visited %d symbols after stop, want 1", count)
	}
}
