package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func resolveDoc(t *testing.T, name, src string) *Resolver {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := New(idx)
	r.ResolveDocument(name, root)
	return r
}

func TestResolveDocumentReportsUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { alias A for P::Missing; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for P::Missing")
	}
}

func TestResolveDocumentCleanWhenAllResolve(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { namespace N; alias A for P::N; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveDocumentResolvesExpressionRefs(t *testing.T) {
	// FilterMember condition referencing an undefined name -> diagnostic.
	r := resolveDoc(t, "d.sysml",
		"package P { filter Undefined; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for expression ref Undefined")
	}
}
