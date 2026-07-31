package semantics

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// buildModel parses src, indexes and resolves it, and returns the semantic
// model plus the document root scope for symbol lookups.
func buildModel(t *testing.T, src string) (*Model, *symbols.Scope) {
	t.Helper()
	const name = "t.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := resolve.New(idx)
	r.ResolveDocument(name, root)
	return NewModel(r), idx.DocumentRoot(name)
}

// sym looks up the first symbol named key at the document root scope.
func sym(t *testing.T, scope *symbols.Scope, key string) *symbols.Symbol {
	t.Helper()
	s, ok := scope.LookupLocal(key)
	if !ok {
		t.Fatalf("symbol %q not found", key)
	}
	return s
}

func TestDirectSupertypes(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A;")
	a := sym(t, root, "A")
	b := sym(t, root, "B")
	supers := m.DirectSupertypes(b)
	if len(supers) != 1 || supers[0] != a {
		t.Fatalf("DirectSupertypes(B) = %v, want [A]", supers)
	}
	if len(m.DirectSupertypes(a)) != 0 {
		t.Fatalf("DirectSupertypes(A) should be empty")
	}
}

func TestDirectSupertypesMemoizedAndDeduped(t *testing.T) {
	// A specializes B twice (odd but legal syntax); dedupe to one edge.
	m, root := buildModel(t, "part def B; part def A specializes B, B;")
	a := sym(t, root, "A")
	b := sym(t, root, "B")
	supers := m.DirectSupertypes(a)
	if len(supers) != 1 || supers[0] != b {
		t.Fatalf("DirectSupertypes(A) = %v, want [B] (deduped)", supers)
	}
	// Second call returns the memoized slice.
	if got := m.DirectSupertypes(a); len(got) != 1 || got[0] != b {
		t.Fatalf("memoized DirectSupertypes(A) = %v", got)
	}
}

func TestAllSupertypesTransitive(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A; part def C specializes B;")
	a := sym(t, root, "A")
	b := sym(t, root, "B")
	c := sym(t, root, "C")
	all := m.AllSupertypes(c)
	if len(all) != 2 {
		t.Fatalf("AllSupertypes(C) = %v, want 2 entries", all)
	}
	got := map[*symbols.Symbol]bool{all[0]: true, all[1]: true}
	if !got[a] || !got[b] {
		t.Fatalf("AllSupertypes(C) missing A or B: %v", all)
	}
}

func TestConforms(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A; part def C specializes B; part def X;")
	a := sym(t, root, "A")
	c := sym(t, root, "C")
	x := sym(t, root, "X")
	if !m.Conforms(c, a) {
		t.Fatalf("C should conform to A")
	}
	if !m.Conforms(a, a) {
		t.Fatalf("A should conform to itself")
	}
	if m.Conforms(a, c) {
		t.Fatalf("A should not conform to C")
	}
	if m.Conforms(x, a) {
		t.Fatalf("unrelated X should not conform to A")
	}
}

func TestConformsAcrossUsageTyping(t *testing.T) {
	// A usage typed by a def conforms to that def.
	m, root := buildModel(t, "part def Engine; part e : Engine;")
	engine := sym(t, root, "Engine")
	e := sym(t, root, "e")
	if !m.Conforms(e, engine) {
		t.Fatalf("usage e should conform to its type Engine")
	}
}

func TestNoCycleOnAcyclicGraph(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A;")
	if m.HasSpecializationCycle(sym(t, root, "A")) || m.HasSpecializationCycle(sym(t, root, "B")) {
		t.Fatalf("no cycle expected in acyclic graph")
	}
}

func TestDetectsTwoNodeCycle(t *testing.T) {
	m, root := buildModel(t, "part def A specializes B; part def B specializes A;")
	if !m.HasSpecializationCycle(sym(t, root, "A")) {
		t.Fatalf("expected cycle A<->B detected from A")
	}
	if !m.HasSpecializationCycle(sym(t, root, "B")) {
		t.Fatalf("expected cycle A<->B detected from B")
	}
}

func TestDetectsThreeNodeCycle(t *testing.T) {
	m, root := buildModel(t,
		"part def A specializes C; part def B specializes A; part def C specializes B;")
	for _, n := range []string{"A", "B", "C"} {
		if !m.HasSpecializationCycle(sym(t, root, n)) {
			t.Fatalf("expected 3-node cycle detected from %s", n)
		}
	}
}

func TestDetectsSelfSpecialization(t *testing.T) {
	m, root := buildModel(t, "part def A specializes A;")
	if !m.HasSpecializationCycle(sym(t, root, "A")) {
		t.Fatalf("expected self-specialization cycle")
	}
}

func TestAllSupertypesSafeOnCycle(t *testing.T) {
	// Must terminate (not infinite-loop) on a cyclic graph.
	m, root := buildModel(t, "part def A specializes B; part def B specializes A;")
	_ = m.AllSupertypes(sym(t, root, "A"))
	_ = m.Conforms(sym(t, root, "A"), sym(t, root, "B"))
}
