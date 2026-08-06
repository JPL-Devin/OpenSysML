package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func addDoc(t *testing.T, idx *Index, name, src string) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	idx.AddDocument(name, root)
}

func TestIndexQualifiedLookup(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { package Q { namespace N; } }")

	syms := idx.LookupQualified("P::Q::N")
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(P::Q::N) len = %d, want 1", len(syms))
	}
	if syms[0].Kind != SymbolNamespace {
		t.Fatalf("P::Q::N kind = %v, want namespace", syms[0].Kind)
	}
	if len(idx.LookupQualified("P::Missing")) != 0 {
		t.Fatalf("LookupQualified(P::Missing) should be empty")
	}
}

func TestIndexAmbiguousQualified(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace D; }")
	addDoc(t, idx, "b.sysml", "package P { namespace D; }")

	if got := len(idx.LookupQualified("P::D")); got != 2 {
		t.Fatalf("LookupQualified(P::D) len = %d, want 2 (ambiguous)", got)
	}
}

// A member a namespace declares shadows one of the same name that a wildcard
// import re-exports through it, so the qualified name stays unambiguous — the
// pattern of SI::min, which is SI's own minute and not an imported function.
func TestIndexOwnedMemberShadowsWildcardReexport(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package Functions { calc def min; calc def clamp; }")
	addDoc(t, idx, "b.sysml", "package Units { public import Functions::*; attribute <min> minute; }")
	idx.ExpandWildcardImports()

	syms := idx.LookupQualified("Units::min")
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(Units::min) len = %d, want 1", len(syms))
	}
	if syms[0].Name != "minute" {
		t.Errorf("Units::min = %q, want the declared minute", syms[0].Name)
	}
	if got := len(idx.LookupQualified("Units::clamp")); got != 1 {
		t.Errorf("LookupQualified(Units::clamp) len = %d, want 1: "+
			"a re-export stays visible when nothing shadows it", got)
	}
}

func TestIndexDocumentRoot(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P;")
	rs := idx.DocumentRoot("a.sysml")
	if rs == nil {
		t.Fatalf("DocumentRoot(a.sysml) = nil")
	}
	if _, ok := rs.LookupLocal("P"); !ok {
		t.Fatalf("document root missing P")
	}
	if idx.DocumentRoot("missing.sysml") != nil {
		t.Fatalf("DocumentRoot(missing) should be nil")
	}
}

func TestIndexShortNameNotDuplicatedInFQN(t *testing.T) {
	// A package with both short and primary names registers one symbol; the
	// FQN uses the primary name. Both local keys still resolve via the scope.
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package <p> Primary { namespace N; }")
	if len(idx.LookupQualified("Primary::N")) != 1 {
		t.Fatalf("Primary::N not indexed")
	}
}

func TestIndexRemoveDocument(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace N; }")
	if got := idx.LookupQualified("P::N"); len(got) != 1 {
		t.Fatalf("before remove: P::N = %d symbols, want 1", len(got))
	}
	idx.RemoveDocument("a.sysml")
	if got := idx.LookupQualified("P::N"); len(got) != 0 {
		t.Fatalf("after remove: P::N = %d symbols, want 0", len(got))
	}
	if idx.DocumentRoot("a.sysml") != nil {
		t.Fatalf("after remove: DocumentRoot should be nil")
	}
}

func TestIndexReAddReplacesStaleEntries(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace Old; }")
	addDoc(t, idx, "a.sysml", "package P { namespace New; }")
	if got := idx.LookupQualified("P::Old"); len(got) != 0 {
		t.Fatalf("P::Old = %d symbols after re-add, want 0 (stale)", len(got))
	}
	if got := idx.LookupQualified("P::New"); len(got) != 1 {
		t.Fatalf("P::New = %d symbols after re-add, want 1", len(got))
	}
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d symbols after re-add, want 1 (not doubled)", len(got))
	}
}

func TestIndexRemoveUnknownDocumentNoop(t *testing.T) {
	idx := NewIndex()
	idx.RemoveDocument("missing.sysml") // must not panic
	addDoc(t, idx, "a.sysml", "package P;")
	idx.RemoveDocument("b.sysml") // unrelated doc untouched
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d after removing unrelated doc, want 1", len(got))
	}
}
