package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// DeclaresVariation reports whether sym is declared with the `variation`
// modifier, which makes it a variation point: an abstract classifier of the
// variants declared for it (SysML v2 §7.20, VariantMembership).
func DeclaresVariation(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.IsVariation
	case *ast.Usage:
		return d.IsVariation
	}
	return false
}

// DeclaresVariant reports whether sym is declared with the `variant` keyword,
// which makes it one of the choices of the variation that owns it.
func DeclaresVariant(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsVariant
}

// VariantValue returns the value expression a variant declares, or nil when it
// declares none and stands for an object of itself.
func VariantValue(sym *symbols.Symbol) ast.Node {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	return usage.Value
}

// VariationOwning returns the variation sym is a variant of — the declaration
// owning the variant membership — or nil when sym is not a variant.
func VariationOwning(sym *symbols.Symbol) *symbols.Symbol {
	if !DeclaresVariant(sym) || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !DeclaresVariation(owner) {
		return nil
	}
	return owner
}

// VariationPointOwning returns the variation point sym is a variant of, or nil
// when sym is not a variant of one: unlike VariationOwning it accepts an owner
// that is a variation by specialization without restating the modifier.
func (m *Model) VariationPointOwning(sym *symbols.Symbol) *symbols.Symbol {
	if !DeclaresVariant(sym) || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !m.IsVariationFeature(owner) {
		return nil
	}
	return owner
}

// IsVariationFeature reports whether sym is a variation point: declared
// `variation` itself, or specializing one — a usage typed by a variation
// definition and a usage redefining a variation usage are both variation
// points, and neither restates the modifier.
func (m *Model) IsVariationFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if DeclaresVariation(sym) {
		return true
	}
	for _, sup := range m.AllSupertypes(sym) {
		if DeclaresVariation(sup) {
			return true
		}
	}
	return false
}

// VariantsOf returns the variants sym offers, in declaration order: those
// declared for it and those it inherits from the variation it specializes. A
// `variant` inherited from a type that is not a variation point offers no choice,
// so it is an ordinary member here too.
func (m *Model) VariantsOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range m.MembersOf(sym) {
		if m.VariationPointOwning(member) != nil {
			out = append(out, member)
		}
	}
	return out
}

// VariantOf returns the variant of sym named name, and whether sym offers one.
func (m *Model) VariantOf(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	for _, variant := range m.VariantsOf(sym) {
		if variant.Name == name {
			return variant, true
		}
	}
	return nil, false
}

// SelectsVariantOf reports whether variant is a variant sym may be bound to:
// one declared for sym itself, or for a variation sym specializes — a usage
// redefining a variation selects among the variants of what it redefines.
func (m *Model) SelectsVariantOf(sym, variant *symbols.Symbol) bool {
	owning := m.VariationPointOwning(variant)
	if owning == nil || sym == nil {
		return false
	}
	if owning == sym {
		return true
	}
	for _, sup := range m.AllSupertypes(sym) {
		if sup == owning {
			return true
		}
	}
	return false
}
