package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// ident is a throwaway AST node used only as a memo key for ResolveName.
func ident(name string) *ast.QualifiedName {
	return qn(false, name)
}

func TestNamespaceImportSkipsPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { public namespace Pub; private namespace Sec; }",
		"app.sysml": "package App { import Lib::*; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := New(idx)
	if _, ok := r.ResolveName(app, "Pub", ident("Pub")); !ok {
		t.Fatalf("expected public member Pub to be importable via Lib::*")
	}
	r2 := New(idx)
	if _, ok := r2.ResolveName(app, "Sec", ident("Sec")); ok {
		t.Fatalf("expected private member Sec to be hidden through namespace import")
	}
}

func TestImportAllReExportsPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { private namespace Sec; }",
		"app.sysml": "package App { import all Lib::*; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := New(idx)
	if _, ok := r.ResolveName(app, "Sec", ident("Sec")); !ok {
		t.Fatalf("expected 'import all' to re-export private member Sec")
	}
}

// A recursive membership import (`import X::**`) walks the subtree of the
// imported member, and every name it surfaces is subject to the same visibility
// filter as a namespace import: private members stay hidden without `all`.
func TestRecursiveMembershipImportSkipsPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { part def Outer { public part def Deep; private part def DeepSec; } }",
		"app.sysml": "package App { import Lib::Outer::**; }",
		"all.sysml": "package AllApp { import all Lib::Outer::**; }",
	})

	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	r := New(idx)
	if _, ok := r.ResolveName(app, "Deep", ident("Deep")); !ok {
		t.Fatalf("expected public nested member Deep to be importable via Lib::Outer::**")
	}
	r2 := New(idx)
	if _, ok := r2.ResolveName(app, "DeepSec", ident("DeepSec")); ok {
		t.Fatalf("expected private nested member DeepSec to be hidden through a recursive membership import")
	}

	allApp := scopeOf(t, idx.DocumentRoot("all.sysml"), "AllApp")
	r3 := New(idx)
	if _, ok := r3.ResolveName(allApp, "DeepSec", ident("DeepSec")); !ok {
		t.Fatalf("expected 'import all' to re-export the private nested member DeepSec")
	}
}

// A hidden name the subtree walk meets first must not end the search: a visible
// member of the same name elsewhere in the subtree is still importable.
func TestRecursiveImportSkipsHiddenNameTwin(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { part def Outer { private part def X; part def Inner { public part def X; } } }",
		"app.sysml": "package App { import Lib::Outer::**; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := New(idx)
	sym, ok := r.ResolveName(app, "X", ident("X"))
	if !ok {
		t.Fatalf("public Inner::X must be importable even though private Outer::X shares its name")
	}
	if sym.Visibility == ast.VisibilityPrivate {
		t.Fatalf("resolved the private X, want the public one")
	}
}

func TestQualifiedAccessIgnoresPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { private namespace Sec; }",
	})
	root := idx.DocumentRoot("lib.sysml")

	r := New(idx)
	if _, ok := r.ResolveQualified(root, qn(false, "Lib", "Sec")); !ok {
		t.Fatalf("expected qualified path Lib::Sec to resolve even though Sec is private")
	}
}
