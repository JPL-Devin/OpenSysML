package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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

// A name a namespace imported privately is not re-exported by it (KerML
// 8.2.3.3), on the unqualified route as much as the qualified one: importing
// Mid::* does not surface what Mid imported privately, while a reference inside
// Mid still reaches it.
func TestNamespaceImportSkipsAPrivatelyImportedName(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"base.sysml": "package Base { part def Hidden; }",
		"mid.sysml":  "package Mid { private import Base::*; }",
		"app.sysml":  "package App { import Mid::*; }",
		"all.sysml":  "package AllApp { import all Mid::*; }",
	})
	idx.ExpandWildcardImports()

	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	r := New(idx)
	if sym, ok := r.ResolveName(app, "Hidden", ident("Hidden")); ok {
		t.Fatalf("Hidden resolved to %q through Mid::*, but Mid imported Base privately",
			sym.Name)
	}

	mid := scopeOf(t, idx.DocumentRoot("mid.sysml"), "Mid")
	r2 := New(idx)
	if _, ok := r2.ResolveName(mid, "Hidden", ident("Hidden")); !ok {
		t.Fatalf("Hidden must stay visible inside Mid; diags=%v", r2.Diagnostics)
	}

	// `import all` takes the target's private memberships, a privately
	// re-exported name among them.
	allApp := scopeOf(t, idx.DocumentRoot("all.sysml"), "AllApp")
	r3 := New(idx)
	if _, ok := r3.ResolveName(allApp, "Hidden", ident("Hidden")); !ok {
		t.Fatalf("`import all Mid::*` must reach Mid's private memberships; diags=%v",
			r3.Diagnostics)
	}
}

// The same, for symbols restored from the library cache: they carry no scope, so
// a wildcard import enumerates them through the index alone.
func TestNamespaceImportSkipsAPrivatelyImportedCachedName(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddRecords("lib", []symbols.RecordEntry{
		{FQN: "Base", Kind: symbols.SymbolPackage},
		{FQN: "Base::Hidden", Kind: symbols.SymbolPartDef},
		{FQN: "Mid", Kind: symbols.SymbolPackage, WildcardImports: []symbols.WildcardImport{
			{Target: "Base", Private: true},
		}},
	})
	idx.AddDocument("app.sysml", parsedRoot(t, "app.sysml", "package App { import Mid::*; }"))
	idx.ExpandWildcardImports()

	appScope := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	r := New(idx)
	if sym, ok := r.ResolveName(appScope, "Hidden", ident("Hidden")); ok {
		t.Fatalf("Hidden resolved to %q through Mid::* from the cached index", sym.Name)
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
