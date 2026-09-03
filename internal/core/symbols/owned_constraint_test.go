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

// An unnamed `require constraint { ... }` and a `require r { ... }` referencing a
// requirement declare no member: their bodies stay local to them.
func TestBuildUnnamedConstraintBodiesStayLocal(t *testing.T) {
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
	}`)
	r := scope.LookupLocalAll("P")[0].Scope.LookupLocalAll("R")[0]
	if got := len(r.Scope.LookupLocalAll("x")); got != 1 {
		t.Fatalf("R declares %d symbols named x, want 1", got)
	}
	if len(r.Scope.LookupLocalAll("bodyB")) != 0 {
		t.Error("bodyB leaked into R")
	}
	var anonymous int
	r.Scope.ForEachMember(func(sym *Symbol) bool {
		if sym.Kind == SymbolConstraintUsage {
			anonymous++
		}
		return true
	})
	if anonymous != 0 {
		t.Errorf("R has %d constraint-usage members, want none", anonymous)
	}
	for _, child := range r.Scope.Children() {
		if child.Owner() == nil && !child.BodyLocal() {
			t.Error("an ownerless body scope is not body-local")
		}
	}
}
