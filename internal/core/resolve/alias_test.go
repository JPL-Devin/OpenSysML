package resolve

import "testing"

func TestAliasResolvesTarget(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Real; alias A for P::Real; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	aSym, ok := pScope.LookupLocal("A")
	if !ok {
		t.Fatalf("alias A not found")
	}
	target, ok := r.ResolveAliasTarget(aSym)
	if !ok {
		t.Fatalf("alias A target unresolved; diags=%v", r.Diagnostics)
	}
	if target.Name != "Real" {
		t.Fatalf("alias target = %q, want Real", target.Name)
	}
}

func TestAliasTransitive(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Real; alias A for P::Real; alias B for P::A; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	bSym, _ := pScope.LookupLocal("B")
	target, ok := r.ResolveAliasTarget(bSym)
	if !ok {
		t.Fatalf("transitive alias B unresolved; diags=%v", r.Diagnostics)
	}
	if target.Name != "Real" {
		t.Fatalf("transitive target = %q, want Real", target.Name)
	}
}

func TestAliasCycleGuard(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { alias A for P::B; alias B for P::A; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	aSym, _ := pScope.LookupLocal("A")
	if _, ok := r.ResolveAliasTarget(aSym); ok {
		t.Fatalf("cyclic alias should not resolve")
	}
}

func TestResolveAliasTargetNonAlias(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package P { namespace Real; }"})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	realSym, _ := pScope.LookupLocal("Real")
	target, ok := r.ResolveAliasTarget(realSym)
	if !ok || target != realSym {
		t.Fatalf("non-alias symbol should resolve to itself")
	}
}
