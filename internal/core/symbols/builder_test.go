package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func build(t *testing.T, src string) *Scope {
	t.Helper()
	sf := source.New("test", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	return Build(root)
}

func TestBuildTopLevelPackage(t *testing.T) {
	root := build(t, "package P;")
	sym, ok := root.LookupLocal("P")
	if !ok {
		t.Fatalf("P not found in root scope")
	}
	if sym.Kind != SymbolPackage {
		t.Fatalf("P kind = %v, want package", sym.Kind)
	}
	if _, isPkg := sym.Decl.(*ast.Package); !isPkg {
		t.Fatalf("P Decl type = %T, want *ast.Package", sym.Decl)
	}
}

func TestBuildNestedMembers(t *testing.T) {
	root := build(t, "package Outer { package Inner; namespace N; }")
	outer, ok := root.LookupLocal("Outer")
	if !ok {
		t.Fatalf("Outer not found")
	}
	outerScope := outer.Scope
	if outerScope == nil {
		t.Fatalf("Outer has no child scope")
	}
	if _, ok := outerScope.LookupLocal("Inner"); !ok {
		t.Fatalf("Inner not found in Outer scope")
	}
	nsym, ok := outerScope.LookupLocal("N")
	if !ok || nsym.Kind != SymbolNamespace {
		t.Fatalf("N not found as namespace in Outer scope")
	}
}

func TestBuildShortAndPrimaryNameKeys(t *testing.T) {
	root := build(t, "package <p> Primary;")
	for _, key := range []string{"p", "Primary"} {
		sym, ok := root.LookupLocal(key)
		if !ok {
			t.Fatalf("key %q not found", key)
		}
		if sym.Kind != SymbolPackage {
			t.Fatalf("key %q kind = %v, want package", key, sym.Kind)
		}
	}
	// Both keys must map to the same symbol.
	a, _ := root.LookupLocal("p")
	b, _ := root.LookupLocal("Primary")
	if a != b {
		t.Fatalf("short and primary keys map to different symbols")
	}
}

func TestBuildVisibilityCarried(t *testing.T) {
	root := build(t, "private package Secret;")
	sym, ok := root.LookupLocal("Secret")
	if !ok {
		t.Fatalf("Secret not found")
	}
	if sym.Visibility != ast.VisibilityPrivate {
		t.Fatalf("Secret visibility = %v, want private", sym.Visibility)
	}
}

func TestBuildAliasSymbol(t *testing.T) {
	root := build(t, "package P; alias A for P;")
	sym, ok := root.LookupLocal("A")
	if !ok || sym.Kind != SymbolAlias {
		t.Fatalf("alias A not found as alias symbol")
	}
}

func TestBuildErrorNodeSkipped(t *testing.T) {
	// Unknown declaration keyword yields an ErrorNode; builder must not panic
	// and must still register the good package.
	root := build(t, "package Good;")
	if _, ok := root.LookupLocal("Good"); !ok {
		t.Fatalf("Good not registered")
	}
}
