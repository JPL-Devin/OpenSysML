package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// visibleNames counts each name a type offers, so a masked inherited feature is
// distinguishable from one that is merely shadowed by a namesake.
func visibleNames(m *Model, sym *symbols.Symbol) map[string]int {
	out := map[string]int{}
	for _, s := range m.MembersOf(sym) {
		out[s.Name]++
		if s.ShortName != "" {
			out[s.ShortName]++
		}
	}
	return out
}

func TestRedefinitionMasksTheRedefinedName(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 {
		t.Fatalf("redefined `a` still visible in B: %v", names)
	}
	if names["b"] != 1 {
		t.Fatalf("redefining `b` should be visible once: %v", names)
	}
}

func TestRedefinitionMasksTheShortNameToo(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part <a_id> a; } part def B specializes A { part b redefines a_id; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 || names["a_id"] != 0 {
		t.Fatalf("both names of the redefined feature must be masked: %v", names)
	}
}

func TestRedefinitionByShortNameMasksTheRegularName(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part <a_id> a; } part def B specializes A { part <b_id> b redefines a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 || names["a_id"] != 0 {
		t.Fatalf("naming the redefined feature by either name masks both: %v", names)
	}
	if names["b"] != 1 || names["b_id"] != 1 {
		t.Fatalf("the redefining feature keeps both of its names: %v", names)
	}
}

func TestRedefinitionMasksTransitivelyRedefinedFeatures(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }"+
			" part def C specializes B { part c redefines b; }")
	names := visibleNames(m, sym(t, root, "C"))
	if names["a"] != 0 || names["b"] != 0 {
		t.Fatalf("a redefinition chain masks every link: %v", names)
	}
	if names["c"] != 1 {
		t.Fatalf("redefining `c` should be visible once: %v", names)
	}
}

func TestRedefinitionMasksOnlyTheFeatureItNames(t *testing.T) {
	// Two inherited features carry the name `a`; only one is redefined, so the
	// other keeps its name and `a` stays visible (masking is by element).
	m, root := buildModel(t,
		"part def A1 { part a; } part def A2 { part a; }"+
			" part def B specializes A1, A2 { part b redefines A1::a; }")
	b := sym(t, root, "B")
	a1 := sym(t, root, "A1")
	a2 := sym(t, root, "A2")
	a1a, _ := a1.Scope.LookupLocal("a")
	a2a, _ := a2.Scope.LookupLocal("a")
	if !m.InheritanceMasked(b, a1a) {
		t.Fatalf("A1::a is redefined and must be masked in B")
	}
	if m.InheritanceMasked(b, a2a) {
		t.Fatalf("A2::a is not redefined and must stay visible in B")
	}
	if visibleNames(m, b)["a"] != 1 {
		t.Fatalf("the surviving namesake keeps the name `a`: %v", visibleNames(m, b))
	}
}

func TestSubsettingDoesNotMask(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b subsets a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 1 {
		t.Fatalf("subsetting is not redefinition and masks nothing: %v", names)
	}
}

func TestRedefinitionKeepingTheNameMasksNothing(t *testing.T) {
	// `part :>> a` is visible as `a` itself, so the name survives.
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part :>> a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 1 {
		t.Fatalf("a redefinition taking the redefined name keeps it visible once: %v", names)
	}
}

func TestMaskedFeatureStaysInTheUnmaskedView(t *testing.T) {
	// Execution binds the redefining and redefined features to one value, so
	// the runtime view keeps both.
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }")
	names := map[string]bool{}
	for _, s := range m.MembersOfIncludingRedefined(sym(t, root, "B")) {
		names[s.Name] = true
	}
	if !names["a"] || !names["b"] {
		t.Fatalf("unmasked view must keep both features: %v", names)
	}
}

func TestInheritedMembersViewOmitsOwnDeclarationsAndTheirMasks(t *testing.T) {
	// Resolving `b redefines a` sees `a`: inside its own declaration B offers
	// what it inherits, not what it declares (KerML 8.3.3.3.6).
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }")
	names := map[string]bool{}
	for _, s := range m.InheritedMembersOf(sym(t, root, "B")) {
		names[s.Name] = true
	}
	if !names["a"] {
		t.Fatalf("the redefinition target must be visible to its own declaration: %v", names)
	}
	if names["b"] {
		t.Fatalf("the declaration being written is not yet a member: %v", names)
	}
}

func TestRedefinitionMasksAnAliasOfTheRedefinedFeature(t *testing.T) {
	// An alias binds a second name to the same element, and masking is by
	// element: naming either name removes both from the inheriting type.
	m, root := buildModel(t,
		"part def A { part a; alias aa for a; } part def B specializes A { part b redefines aa; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 || names["aa"] != 0 {
		t.Fatalf("the redefined element and its alias must both be masked: %v", names)
	}
}

func TestRedefinitionMasksTheNameOnlyAtItsOwnLevel(t *testing.T) {
	// Masking is a namespace question: `a` is gone from B, and the nested names
	// under the redefining feature are its own, reached through it.
	m, root := buildModel(t,
		"part def A { part a { part x; } }"+
			" part def B specializes A { part b redefines a { part y; } }")
	b := sym(t, root, "B")
	if visibleNames(m, b)["a"] != 0 {
		t.Fatalf("`a` must not be a member name of B: %v", visibleNames(m, b))
	}
	bb, ok := b.Scope.LookupLocal("b")
	if !ok {
		t.Fatalf("B declares b")
	}
	nested := visibleNames(m, bb)
	if nested["y"] != 1 || nested["x"] != 1 {
		t.Fatalf("b offers its own y and the inherited x of the feature it redefines: %v", nested)
	}
}
