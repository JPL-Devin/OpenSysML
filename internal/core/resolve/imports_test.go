package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestImportMembership(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; }",
		"b.sysml": "package App { import Lib::Widget; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	sym, ok := r.ResolveName(appScope, "Widget", &ast.FeatureReference{})
	if !ok {
		t.Fatalf("Widget unresolved via membership import; diags=%v", r.Diagnostics)
	}
	if sym.Name != "Widget" {
		t.Fatalf("resolved %q, want Widget", sym.Name)
	}
}

func TestImportNamespaceStar(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; namespace Gadget; }",
		"b.sysml": "package App { import Lib::*; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Widget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Widget unresolved via namespace import; diags=%v", r.Diagnostics)
	}
	if _, ok := r.ResolveName(appScope, "Gadget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Gadget unresolved via namespace import")
	}
}

func TestImportRecursiveMembership(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Outer { namespace Deep; } }",
		"b.sysml": "package App { import Lib::Outer::**; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Outer", &ast.FeatureReference{}); !ok {
		t.Fatalf("Outer unresolved via recursive import; diags=%v", r.Diagnostics)
	}
	if _, ok := r.ResolveName(appScope, "Deep", &ast.FeatureReference{}); !ok {
		t.Fatalf("Deep unresolved via recursive import")
	}
}

func TestImportDoesNotLeakNonImported(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; namespace Hidden; }",
		"b.sysml": "package App { import Lib::Widget; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Hidden", &ast.FeatureReference{}); ok {
		t.Fatalf("Hidden should NOT be visible (only Widget imported)")
	}
}
