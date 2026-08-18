package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// An entry/do/exit action a state declares by name is a feature of that state,
// as the standard library's StateAction relies on, so it must be a member of
// the state's scope and not disappear into the parser's entry/do/exit wrapper.
func TestBuildNamedEntryDoExitActionsAreStateMembers(t *testing.T) {
	scope := buildScope(t, `abstract state def StateAction {
		doc
		/* base */

		entry action entryAction :>> 'entry';
		do action doAction :>> 'do';
		exit action exitAction :>> 'exit';
	}`)

	syms := scope.LookupLocalAll("StateAction")
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for StateAction, got %d", len(syms))
	}
	state := syms[0].Scope
	if state == nil {
		t.Fatal("StateAction has no scope")
	}
	for _, name := range []string{"entryAction", "doAction", "exitAction"} {
		if got := len(state.LookupLocalAll(name)); got != 1 {
			t.Errorf("StateAction declares %d symbols named %q, want 1", got, name)
		}
	}
}
