package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func buildScope(t *testing.T, src string) *Scope {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	return Build(root)
}

func TestBuildNewKindSymbols(t *testing.T) {
	cases := []struct {
		src  string
		name string
		want SymbolKind
	}{
		{"item def Widget;", "Widget", SymbolItemDef},
		{"port def P;", "P", SymbolPortDef},
		{"connection def C;", "C", SymbolConnectionDef},
		{"action def A;", "A", SymbolActionDef},
		{"use case def Login;", "Login", SymbolUseCaseDef},
		{"item w;", "w", SymbolItemUsage},
		{"port p;", "p", SymbolPortUsage},
		{"use case checkout;", "checkout", SymbolUseCaseUsage},
	}
	for _, c := range cases {
		scope := buildScope(t, c.src)
		syms := scope.LookupLocalAll(c.name)
		if len(syms) != 1 {
			t.Fatalf("%q: expected 1 symbol for %q, got %d", c.src, c.name, len(syms))
		}
		if syms[0].Kind != c.want {
			t.Fatalf("%q: kind = %v, want %v", c.src, syms[0].Kind, c.want)
		}
	}
}

func TestBuildConnectorEndsDefineNoSymbols(t *testing.T) {
	// Connector-end references must not register any symbols in the enclosing
	// scope beyond the connection usage itself.
	scope := buildScope(t, "part def Sys { part a; part b; connection c connect a to b; }")
	sys := scope.LookupLocalAll("Sys")[0]
	// a, b, c only (ends contribute nothing).
	for _, name := range []string{"a", "b", "c"} {
		if len(sys.Scope.LookupLocalAll(name)) != 1 {
			t.Fatalf("expected %q registered exactly once", name)
		}
	}
	if got := len(sys.Scope.Children()); got != 3 {
		t.Fatalf("expected 3 child scopes (a, b, c), got %d", got)
	}
}
