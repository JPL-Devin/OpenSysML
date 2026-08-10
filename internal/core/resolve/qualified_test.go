package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func indexOf(t *testing.T, docs map[string]string) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	for name, src := range docs {
		sf := source.New(name, []byte(src))
		p := parser.New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics for %s: %v", name, p.Diagnostics)
		}
		idx.AddDocument(name, root)
	}
	return idx
}

func qn(global bool, parts ...string) *ast.QualifiedName {
	q := &ast.QualifiedName{Global: global}
	for _, p := range parts {
		q.Parts = append(q.Parts, ast.NameSegment{Text: p})
	}
	return q
}

func TestResolveQualifiedFromRoot(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { package Q { namespace N; } }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	sym, ok := r.ResolveQualified(root, qn(false, "P", "Q", "N"))
	if !ok {
		t.Fatalf("P::Q::N unresolved; diagnostics: %v", r.Diagnostics)
	}
	if sym.Kind != symbols.SymbolNamespace {
		t.Fatalf("P::Q::N kind = %v, want namespace", sym.Kind)
	}
}

func TestResolveQualifiedMissingSegment(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { package Q; }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	if _, ok := r.ResolveQualified(root, qn(false, "P", "Missing")); ok {
		t.Fatalf("P::Missing should be unresolved")
	}
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic")
	}
}

func TestResolveQualifiedGlobal(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace N; }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	// $::P::N resolves from the document root regardless of scope.
	sym, ok := r.ResolveQualified(root, qn(true, "P", "N"))
	if !ok || sym.Kind != symbols.SymbolNamespace {
		t.Fatalf("$::P::N unresolved or wrong kind: %v ok=%v", sym, ok)
	}
}

// A name a namespace holds only through a private wildcard import is not a
// visible member of it, so a qualified reference from another package does not
// reach it — while the same reference made inside that namespace does
// (KerML 8.2.3.3).
func TestResolveQualifiedRejectsAPrivatelyImportedName(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"base.sysml": "package Base { part def Hidden; }",
		"mid.sysml":  "package Mid { private import Base::*; }",
		"app.sysml":  "package App { }",
	})
	idx.ExpandWildcardImports()
	r := New(idx)

	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	if sym, ok := r.ResolveQualified(app, qn(false, "Mid", "Hidden")); ok {
		t.Fatalf("Mid::Hidden resolved to %q from App: Mid imported Base privately",
			sym.Name)
	}

	mid := scopeOf(t, idx.DocumentRoot("mid.sysml"), "Mid")
	if _, ok := r.ResolveQualified(mid, qn(false, "Mid", "Hidden")); !ok {
		t.Fatalf("Mid::Hidden unresolved inside Mid, where the private import is "+
			"visible; diagnostics: %v", r.Diagnostics)
	}
}

// A public wildcard import still re-exports: the qualified reference the private
// case rejects resolves here.
func TestResolveQualifiedReachesAPubliclyImportedName(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"base.sysml": "package Base { part def Shown; }",
		"mid.sysml":  "package Mid { public import Base::*; }",
		"app.sysml":  "package App { }",
	})
	idx.ExpandWildcardImports()
	r := New(idx)

	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	if _, ok := r.ResolveQualified(app, qn(false, "Mid", "Shown")); !ok {
		t.Fatalf("Mid::Shown unresolved from App; diagnostics: %v", r.Diagnostics)
	}
}

func TestResolveQualifiedSegmentIntoLeaf(t *testing.T) {
	// A leaf symbol (no child scope) cannot own further segments.
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { comment C /* x */ }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	if _, ok := r.ResolveQualified(root, qn(false, "P", "C", "Deeper")); ok {
		t.Fatalf("P::C::Deeper should fail past the leaf comment")
	}
}
