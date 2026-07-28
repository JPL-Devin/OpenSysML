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
