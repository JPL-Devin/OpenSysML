package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestW7BRelationshipMetaclassNamesAreDeclared guards the relationship kind →
// metaclass mapping against a name no KerML metaclass package declares.
func TestW7BRelationshipMetaclassNamesAreDeclared(t *testing.T) {
	m := stdlibModel(t)
	names := []string{"Conjugation"}
	for _, name := range relationshipMetaclassNames {
		names = append(names, name)
	}
	for _, name := range names {
		if m.kermlMetaclass(name) == nil {
			t.Errorf("relationship metaclass %s is not declared by any KerML metaclass package", name)
		}
	}
}

// A relationship written keyword-first classifies as its own metaclass, not as
// the usage the superseded representation made of it (KerML §7.2, §8.2.4).
func TestW7BKeywordFirstRelationshipClassifiesAsItsMetaclass(t *testing.T) {
	const src = `
		class A;
		class B;
		specialization Gen subclassifier A specializes B;
		conjugation Conj conjugate A conjugates B;
	`
	m, root := stdlibModelWithDoc(t, "w7b_metaclass.kerml", src)
	for _, tc := range []struct{ name, metaclass string }{
		{"Gen", "Specialization"},
		{"Conj", "Conjugation"},
	} {
		elem := sym(t, root, tc.name)
		got := m.metaclassOf(elem)
		if got == nil {
			t.Fatalf("%s has no metaclass, want %s", tc.name, tc.metaclass)
		}
		if got.Name != tc.metaclass {
			t.Errorf("%s classifies as %s, want %s", tc.name, got.Name, tc.metaclass)
		}
		if _, ok := elem.Decl.(*ast.RelationshipMember); !ok {
			t.Errorf("%s is declared by %T, want a relationship member", tc.name, elem.Decl)
		}
		if !m.metaclassConforms(elem, "KerML::Core::"+tc.metaclass) {
			t.Errorf("%s should conform to the %s metaclass", tc.name, tc.metaclass)
		}
	}
}

// stdlibModelWithDoc resolves src against the standard library, so that a
// declaration can be classified by a library-declared metaclass.
func stdlibModelWithDoc(t *testing.T, name, src string) (*Model, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := stdlibIndex(t)
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	return m, idx.DocumentRoot(name)
}
