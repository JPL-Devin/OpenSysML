package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const requirementMemberShortNameModel = `package P {
	part def T { attribute v; }
	constraint def C;
	requirement def R {
		subject <s> x : T;
		assume constraint <a> ac : C;
		require constraint <r> rc : C;
		require constraint { s.v == s.v and a and r }
	}
	requirement def R2 :> R {
		subject :>> s;
		assume constraint :>> a;
		require constraint :>> r;
	}
	requirement def R3 :> R {
		subject <s2> y :>> R::s;
	}
}`

// The short name of a subject, assume or require member resolves wherever the
// short name of a `part <p>` does: qualified from outside, as the target of a
// redefinition in a specializing requirement, and from an expression.
func TestRequirementMemberShortNamesResolve(t *testing.T) {
	r, root, rootScope := resolvedDoc(t, requirementMemberShortNameModel)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolution diagnostics: %v", r.Diagnostics)
	}
	pkg := local(t, rootScope, "P")
	req := local(t, pkg.Scope, "R")
	want := map[string]*symbols.Symbol{
		"s": local(t, req.Scope, "x"),
		"a": local(t, req.Scope, "ac"),
		"r": local(t, req.Scope, "rc"),
	}

	for short, sym := range want {
		got, ok := r.ResolveQualified(rootScope, qualified("P", "R", short))
		if !ok || got != sym {
			t.Errorf("P::R::%s = %v, %v; want %s", short, got, ok, sym.Name)
		}
	}

	// Every `s`, `a` and `r` written in the model — the redefinitions in R2 and
	// R3 and the operands of the anonymous constraint — names the R member.
	seen := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		if ref.QN == nil {
			continue
		}
		last := ref.QN.Parts[len(ref.QN.Parts)-1].Text
		sym, isShort := want[last]
		if !isShort {
			continue
		}
		got, ok := r.ResolveReference(ref)
		if !ok || got != sym {
			t.Errorf("reference %s at %d = %v, %v; want %s", last, ref.QN.Span().Offset, got, ok, sym.Name)
		}
		seen[last]++
	}
	if seen["s"] != 4 || seen["a"] != 2 || seen["r"] != 2 {
		t.Errorf("references seen = %v, want s:4 a:2 r:2", seen)
	}

	// A redefining constraint is registered under the short name it redefines,
	// as a `part :>> wheel` is; a redefining subject with a short name of its
	// own is one symbol under both its names.
	r2 := local(t, pkg.Scope, "R2")
	for _, name := range []string{"a", "r"} {
		local(t, r2.Scope, name)
	}
	r3 := local(t, pkg.Scope, "R3")
	if local(t, r3.Scope, "s2") != local(t, r3.Scope, "y") {
		t.Error("R3's subject is not one symbol under both its names")
	}
}

func qualified(parts ...string) *ast.QualifiedName {
	q := &ast.QualifiedName{}
	for _, p := range parts {
		q.Parts = append(q.Parts, ast.NameSegment{Text: p})
	}
	return q
}
