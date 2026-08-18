package symbols

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// actionBodyScope returns the scope of the sole action def in src.
func actionBodyScope(t *testing.T, src string) *Scope {
	t.Helper()
	file := parser.New(source.New("test", []byte(src))).ParseFile()
	idx := NewIndex()
	idx.AddDocument("test", file)
	root := idx.DocumentRoot("test")
	if root == nil || len(root.Children()) == 0 {
		t.Fatal("no document root")
	}
	pkg := root.Children()[0]
	if len(pkg.Children()) == 0 {
		t.Fatal("no action def scope")
	}
	return pkg.Children()[0]
}

// TestControlNodeSymbols covers the four control nodes: a named one is a member
// of the action body, recorded as an action usage at its name's span.
func TestControlNodeSymbols(t *testing.T) {
	src := `package P {
	action def F {
		fork Jump;
		join Land;
		merge M;
		decision D;
	}
}`
	scope := actionBodyScope(t, src)
	for _, name := range []string{"Jump", "Land", "M", "D"} {
		sym, _ := scope.LookupLocal(name)
		if sym == nil {
			t.Errorf("%s not registered", name)
			continue
		}
		if sym.Kind != SymbolActionUsage {
			t.Errorf("%s has kind %v, want %v", name, sym.Kind, SymbolActionUsage)
		}
		if want := strings.Index(src, name+";"); sym.NameSpan.Offset != want {
			t.Errorf("%s NameSpan at %d, want %d", name, sym.NameSpan.Offset, want)
		}
	}
}

// TestUnnamedControlNodesRegisterNothing covers unnamed control nodes: they
// declare no name, so the action body gains no member for them.
func TestUnnamedControlNodesRegisterNothing(t *testing.T) {
	src := `package P {
	action def F {
		fork;
		join;
		merge;
		decision;
	}
}`
	if names := actionBodyScope(t, src).MemberNames(); len(names) != 0 {
		t.Errorf("unnamed control nodes registered %v", names)
	}
}
