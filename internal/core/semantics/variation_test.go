package semantics

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// member looks up a member of a symbol, for a variant declared in a variation.
func member(t *testing.T, m *Model, owner *symbols.Symbol, name string) *symbols.Symbol {
	t.Helper()
	got, ok := m.LookupMember(owner, name)
	if !ok {
		t.Fatalf("member %q of %s not found", name, owner.Name)
	}
	return got
}

const variationSource = `
	part def Diamond { attribute cut; }
	abstract part family : Diamond {
		variation attribute :>> cut {
			variant attribute cutShallow { attribute cost = 200.0; }
			variant attribute cutIdeal { attribute cost = 250.0; }
		}
	}
	part chosen :> family { attribute :>> cut = cut::cutIdeal; }
`

func TestVariationAndVariantModifiers(t *testing.T) {
	m, root := buildModel(t, variationSource)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	ideal := member(t, m, cut, "cutIdeal")

	if !DeclaresVariation(cut) {
		t.Error("the redefinition of cut declares `variation`")
	}
	if DeclaresVariant(cut) {
		t.Error("a variation is not itself a variant")
	}
	if !DeclaresVariant(ideal) {
		t.Error("cutIdeal declares `variant`")
	}
	if got := VariationOwning(ideal); got != cut {
		t.Errorf("VariationOwning(cutIdeal) = %v, want cut", got)
	}
	if got := VariationOwning(cut); got != nil {
		t.Errorf("VariationOwning(cut) = %v, want nil", got)
	}
	if VariantValue(ideal) != nil {
		t.Error("cutIdeal declares no value, so it stands for an object of itself")
	}
}

// A variation point is the feature declaring the modifier and any feature
// specializing it, since a redefinition does not restate the modifier.
func TestIsVariationFeatureThroughSpecialization(t *testing.T) {
	m, root := buildModel(t, variationSource)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	chosenCut := member(t, m, sym(t, root, "chosen"), "cut")

	if !m.IsVariationFeature(cut) {
		t.Error("cut declares `variation`")
	}
	if !m.IsVariationFeature(chosenCut) {
		t.Error("the redefinition of cut in chosen is a variation point too")
	}
	if m.IsVariationFeature(member(t, m, sym(t, root, "Diamond"), "cut")) {
		t.Error("the definition's plain cut is not a variation point")
	}
}

// The variants a feature offers are those of the variation it specializes, so a
// specialization selects among the same choices as the family it refines.
func TestVariantsOfInheritedThroughRedefinition(t *testing.T) {
	m, root := buildModel(t, variationSource)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	chosenCut := member(t, m, sym(t, root, "chosen"), "cut")

	for _, owner := range []*symbols.Symbol{cut, chosenCut} {
		variants := m.VariantsOf(owner)
		if len(variants) != 2 ||
			variants[0].Name != "cutShallow" || variants[1].Name != "cutIdeal" {
			t.Fatalf("VariantsOf(%s) = %v, want cutShallow then cutIdeal", owner.Name, variants)
		}
		ideal, ok := m.VariantOf(owner, "cutIdeal")
		if !ok || ideal.Name != "cutIdeal" {
			t.Errorf("VariantOf(%s, cutIdeal) = (%v, %v)", owner.Name, ideal, ok)
		}
		if _, ok := m.VariantOf(owner, "nope"); ok {
			t.Errorf("VariantOf(%s, nope) found a variant", owner.Name)
		}
		if !m.SelectsVariantOf(owner, ideal) {
			t.Errorf("%s may be bound to cutIdeal", owner.Name)
		}
	}
}

// A variant of another variation is not a choice a feature may be bound to.
func TestSelectsVariantOfRejectsForeignVariant(t *testing.T) {
	m, root := buildModel(t, `
		part def Diamond { attribute cut; attribute color; }
		abstract part family : Diamond {
			variation attribute :>> cut {
				variant attribute cutIdeal { attribute cost = 250.0; }
			}
			variation attribute :>> color {
				variant attribute colorWhite { attribute cost = 100.0; }
			}
		}
	`)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	white := member(t, m, member(t, m, family, "color"), "colorWhite")

	if m.SelectsVariantOf(cut, white) {
		t.Error("colorWhite is a variant of color, not of cut")
	}
	if m.SelectsVariantOf(cut, cut) {
		t.Error("a variation is not a variant of itself")
	}
	if m.SelectsVariantOf(nil, white) || m.SelectsVariantOf(cut, nil) {
		t.Error("a missing symbol selects nothing")
	}
}
