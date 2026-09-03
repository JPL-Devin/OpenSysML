package symbols

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// A constraint a requirement declares through `require constraint c : C` or
// `assume constraint a : C` is a constraint usage the requirement owns
// (SysML v2 §7.20.5), so it is a member of the requirement's scope.
func TestBuildNamedOwnedConstraintsAreRequirementMembers(t *testing.T) {
	src := `package P {
		constraint def C;
		requirement def R {
			require constraint c : C;
			attribute x;
			assume constraint a : C { x > 0 }
		}
	}`
	scope := buildScope(t, src)
	req := scope.LookupLocalAll("P")[0].Scope.LookupLocalAll("R")[0]
	for _, tc := range []struct {
		name string
		decl string
	}{
		{"c", "*ast.RequireMember"},
		{"a", "*ast.AssumeMember"},
	} {
		syms := req.Scope.LookupLocalAll(tc.name)
		if len(syms) != 1 {
			t.Fatalf("R declares %d symbols named %q, want 1", len(syms), tc.name)
		}
		sym := syms[0]
		if sym.Kind != SymbolConstraintUsage {
			t.Errorf("%s: kind = %v, want %v", tc.name, sym.Kind, SymbolConstraintUsage)
		}
		if sym.Visibility != ast.VisibilityDefault {
			t.Errorf("%s: visibility = %v, want default", tc.name, sym.Visibility)
		}
		if sym.OwnerScope != req.Scope || sym.Scope == nil || sym.Scope.Owner() != sym {
			t.Errorf("%s: not owned by R with a scope of its own", tc.name)
		}
		if sym.NameSpan.Offset != strings.Index(src, "constraint "+tc.name)+len("constraint ") {
			t.Errorf("%s: NameSpan at %d, want the declared name", tc.name, sym.NameSpan.Offset)
		}
		if got := fmt.Sprintf("%T", sym.Decl); got != tc.decl {
			t.Errorf("%s: declared by %s, want %s", tc.name, got, tc.decl)
		}
	}
	a := req.Scope.LookupLocalAll("a")[0]
	if ConstraintBodyScope(req.Scope, a.Decl) != a.Scope {
		t.Error("a's body resolves outside a's scope")
	}
}

// A constraint that redefines one of the requirement's generals without a
// name of its own answers to the redefined name, as a usage does.
func TestBuildOwnedConstraintBorrowsRedefinedName(t *testing.T) {
	scope := buildScope(t, `package P {
		constraint def C;
		requirement def R { require constraint c : C; }
		requirement def S :> R { require constraint :>> c; }
	}`)
	s := scope.LookupLocalAll("P")[0].Scope.LookupLocalAll("S")[0]
	syms := s.Scope.LookupLocalAll("c")
	if len(syms) != 1 {
		t.Fatalf("S declares %d symbols named c, want 1", len(syms))
	}
	if !syms[0].EffectiveName || syms[0].NamingTarget == nil {
		t.Error("c's name is not recorded as borrowed from its redefinition")
	}
}

// An unnamed `require constraint { ... }` declares an anonymous constraint usage
// the requirement owns, one that redefines two features too (no lone naming
// feature names it); a `require r { ... }` referencing a requirement declares no
// member and its body stays local to it.
func TestBuildUnnamedOwnedConstraintsAreAnonymousMembers(t *testing.T) {
	scope := buildScope(t, `package P {
		constraint def C;
		requirement def Q;
		requirement q : Q;
		requirement def R {
			attribute x;
			require constraint { x > 0 }
			require q { attribute bodyB; }
			assume constraint : C { x > 0 }
		}
		requirement def S :> R {
			require constraint c1 : C;
			require constraint c2 : C;
			require constraint :>> c1, c2;
		}
	}`)
	pkg := scope.LookupLocalAll("P")[0].Scope
	r := pkg.LookupLocalAll("R")[0]
	if got := len(r.Scope.LookupLocalAll("x")); got != 1 {
		t.Fatalf("R declares %d symbols named x, want 1", got)
	}
	if len(r.Scope.LookupLocalAll("bodyB")) != 0 {
		t.Error("bodyB leaked into R")
	}
	anonymousConstraints := func(owner *Symbol) []*Symbol {
		var out []*Symbol
		for _, sym := range owner.Scope.AnonymousMembers() {
			if sym.Kind == SymbolConstraintUsage {
				out = append(out, sym)
			}
		}
		return out
	}
	anon := anonymousConstraints(r)
	if len(anon) != 2 {
		t.Fatalf("R has %d anonymous constraint usages, want 2", len(anon))
	}
	for _, sym := range anon {
		if sym.Name != "" || sym.EffectiveName || sym.OwnerScope != r.Scope || sym.Scope == nil || sym.Scope.Owner() != sym {
			t.Errorf("%T: not an anonymous constraint usage R owns with a scope of its own", sym.Decl)
		}
		if ConstraintBodyScope(r.Scope, sym.Decl) != sym.Scope {
			t.Errorf("%T: body resolves outside its own scope", sym.Decl)
		}
	}
	if _, ok := anon[0].Decl.(*ast.RequireMember); !ok {
		t.Errorf("first anonymous constraint declared by %T, want *ast.RequireMember", anon[0].Decl)
	}
	if _, ok := anon[1].Decl.(*ast.AssumeMember); !ok {
		t.Errorf("second anonymous constraint declared by %T, want *ast.AssumeMember", anon[1].Decl)
	}
	s := pkg.LookupLocalAll("S")[0]
	if anon := anonymousConstraints(s); len(anon) != 1 || anon[0].NamingTarget != nil {
		t.Errorf("S has %d anonymous constraint usages, want the one redefining c1 and c2", len(anon))
	}
	for _, child := range r.Scope.Children() {
		if child.Owner() == nil && !child.BodyLocal() {
			t.Error("an ownerless body scope is not body-local")
		}
	}
}
