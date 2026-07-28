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
