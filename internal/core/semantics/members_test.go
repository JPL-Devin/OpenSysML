package semantics

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func memberNames(syms []*symbols.Symbol) map[string]bool {
	out := make(map[string]bool)
	for _, s := range syms {
		out[s.Name] = true
	}
	return out
}

func TestMembersOfLocalOnly(t *testing.T) {
	m, root := buildModel(t, "part def C { part a; part b; }")
	c := sym(t, root, "C")
	names := memberNames(m.MembersOf(c))
	if !names["a"] || !names["b"] {
		t.Fatalf("MembersOf(C) = %v, want a and b", names)
	}
}

func TestMembersOfInherited(t *testing.T) {
	m, root := buildModel(t,
		"part def Base { part a; } part def Sub specializes Base { part b; }")
	sub := sym(t, root, "Sub")
	names := memberNames(m.MembersOf(sub))
	if !names["a"] || !names["b"] {
		t.Fatalf("MembersOf(Sub) = %v, want inherited a and local b", names)
	}
}

func TestMembersOfMasking(t *testing.T) {
	// Sub redeclares `a`; the local one masks the inherited one (single `a`).
	m, root := buildModel(t,
		"part def Base { part a; } part def Sub specializes Base { part a; }")
	base := sym(t, root, "Base")
	sub := sym(t, root, "Sub")
	baseA, _ := base.Scope.LookupLocal("a")
	subA, _ := sub.Scope.LookupLocal("a")

	members := m.MembersOf(sub)
	count := 0
	var got *symbols.Symbol
	for _, s := range members {
		if s.Name == "a" {
			count++
			got = s
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one visible `a`, got %d", count)
	}
	if got != subA {
		t.Fatalf("local `a` should mask inherited; got the base one")
	}
	if got == baseA {
		t.Fatalf("visible `a` should be the subtype's, not base's")
	}
}

func TestMembersOfTransitiveInheritance(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part x; } part def B specializes A { part y; } part def C specializes B { part z; }")
	c := sym(t, root, "C")
	names := memberNames(m.MembersOf(c))
	for _, n := range []string{"x", "y", "z"} {
		if !names[n] {
			t.Fatalf("MembersOf(C) missing %q: %v", n, names)
		}
	}
}

func TestLookupMemberInherited(t *testing.T) {
	m, root := buildModel(t,
		"part def Base { part a; } part def Sub specializes Base;")
	sub := sym(t, root, "Sub")
	if _, ok := m.LookupMember(sub, "a"); !ok {
		t.Fatalf("LookupMember(Sub, a) should find inherited member")
	}
	if _, ok := m.LookupMember(sub, "nope"); ok {
		t.Fatalf("LookupMember(Sub, nope) should not resolve")
	}
}
