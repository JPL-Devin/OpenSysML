package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func featureTypeNames(m *Model, sym *symbols.Symbol) []string {
	var out []string
	for _, t := range m.FeatureTypes(sym) {
		out = append(out, t.Name)
	}
	return out
}

func wantTypes(t *testing.T, m *Model, sym *symbols.Symbol, want ...string) {
	t.Helper()
	got := featureTypeNames(m, sym)
	if len(got) != len(want) {
		t.Fatalf("FeatureTypes(%s) = %v, want %v", sym.Name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FeatureTypes(%s) = %v, want %v", sym.Name, got, want)
		}
	}
}

// An owned cross feature is a feature: it carries the type it declares, and is
// implicitly typed by its end's types (KerML 1.1 §8.3.3.3
// checkFeatureOwnedCrossFeatureSpecialization).
func TestOwnedCrossFeatureTypes(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class C1; class C2;
		class Sub1 :> C1;
		assoc A {
			end x1 [0..1] feature x : C1;
			end y1 [0..1] typed by Sub1 feature y : C2;
		}
		assoc B {
			end feature p : C1 { member feature p1 [0..1] featured by C2; }
			end feature q : C2;
		}
	}`)
	p := sym(t, root, "P")
	a := nested(t, p.Scope, "A")
	wantTypes(t, m, nested(t, a.Scope, "x", "x1"), "C1")
	wantTypes(t, m, nested(t, a.Scope, "y", "y1"), "Sub1", "C2")
	b := nested(t, p.Scope, "B")
	wantTypes(t, m, nested(t, b.Scope, "p", "p1"), "C1")
	wantTypes(t, m, nested(t, b.Scope, "q"), "C2")
}

// An inline cross feature of an end that redefines another end subsets that
// end's cross feature, so it inherits its type when neither it nor its end
// declares one.
func TestOwnedCrossFeatureInheritsRedefinedCrossFeatureType(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class C1; class C2;
		assoc A {
			end feature x { member feature x1 : C1 [0..1] featured by C2; }
			end y1 [0..1] feature y : C2;
		}
		assoc B :> A {
			end x2 [0..1] feature :>> x;
			end y2 [0..1] feature :>> y;
		}
	}`)
	p := sym(t, root, "P")
	b := nested(t, p.Scope, "B")
	x2 := nested(t, b.Scope, "x", "x2")
	wantTypes(t, m, x2, "C1")
	a := nested(t, p.Scope, "A")
	x1 := nested(t, a.Scope, "x", "x1")
	if got := m.DeclaredFeatureTypes(nested(t, b.Scope, "x")); len(got) != 0 {
		t.Fatalf("end x declares types %v, want none so x2's type can only come from x1", got)
	}
	if !m.Conforms(x2, x1) {
		t.Fatalf("x2 does not specialize the redefined end's cross feature x1: %v", m.DirectSupertypes(x2))
	}
	wantTypes(t, m, nested(t, b.Scope, "y", "y2"), "C2")
}

// The same holds in SysML, where a connection definition's ends declare their
// cross features inline.
func TestOwnedCrossFeatureTypesSysML(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package P {
		part def Person; part def Car;
		connection def Owns {
			end owners [0..*] part owner : Person;
			end cars [0..*] part car : Car;
		}
	}`)
	p := sym(t, root, "P")
	owns := nested(t, p.Scope, "Owns")
	wantTypes(t, m, nested(t, owns.Scope, "owner", "owners"), "Person")
	wantTypes(t, m, nested(t, owns.Scope, "car", "cars"), "Car")
}

// The typing, subsetting and multiplicity written ahead of an end's kind keyword
// belong to its named cross feature (KerML.xtext OwnedCrossingFeature); the end
// keeps only the relationships it states itself.
func TestNamedCrossFeatureRelationshipsStayOnCrossFeature(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"t.kerml", `package P {
			class C1; class C2;
			class Sub1 :> C1;
			feature g : C1;
			assoc A {
				end x1 : Sub1 [0..1] :> g feature x : C1;
				end y1 [0..1] feature y : C2;
			}
		}`},
		{"keywords.kerml", `package P {
			class C1; class C2;
			class Sub1 :> C1;
			feature g : C1;
			assoc A {
				end x1 [0..1] typed by Sub1 subsets g feature x : C1;
				end y1 [0..1] feature y : C2;
			}
		}`},
		{"t.sysml", `package P {
			part def C1; part def C2;
			part def Sub1 :> C1;
			item g : C1;
			connection def A {
				end x1 : Sub1 [0..1] :> g item x : C1;
				end y1 [0..1] item y : C2;
			}
		}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, root := buildModelNamed(t, tc.name, tc.src)
			p := sym(t, root, "P")
			g := nested(t, p.Scope, "g")
			a := nested(t, p.Scope, "A")
			x := nested(t, a.Scope, "x")
			x1 := nested(t, a.Scope, "x", "x1")

			wantRelationshipKinds(t, x, ast.RelTyping)
			wantRelationshipKinds(t, x1, ast.RelTyping, ast.RelSubsets)
			wantTypes(t, m, x, "C1")
			wantTypes(t, m, x1, "Sub1", "C1")
			if got := m.DeclaredFeatureTypes(x); len(got) != 1 || got[0].Name != "C1" {
				t.Fatalf("end x declares types %v, want only C1", got)
			}
			if m.Conforms(x, g) {
				t.Fatalf("end x subsets g, want that subsetting left to x1: %v", m.DirectSupertypes(x))
			}
			if !m.Conforms(x1, g) {
				t.Fatalf("cross feature x1 does not subset g: %v", m.DirectSupertypes(x1))
			}
			if _, ok := m.MultiplicityOf(x); ok {
				t.Fatal("MultiplicityOf(x) ok, want the [0..1] left to x1")
			}
			known := func(v int64) Bound { return Bound{Value: v, Known: true} }
			if r, ok := m.MultiplicityOf(x1); !ok || r != (Range{known(0), known(1)}) {
				t.Fatalf("MultiplicityOf(x1) = %+v, %v, want [0..1]", r, ok)
			}
		})
	}
}

func wantRelationshipKinds(t *testing.T, sym *symbols.Symbol, want ...ast.RelationshipKind) {
	t.Helper()
	rels := RelationshipsOf(sym)
	if len(rels) != len(want) {
		t.Fatalf("RelationshipsOf(%s) has %d relationships, want %v", sym.Name, len(rels), want)
	}
	for i, r := range rels {
		if r.Kind != want[i] {
			t.Fatalf("RelationshipsOf(%s)[%d] = %v, want %v", sym.Name, i, r.Kind, want[i])
		}
	}
}
