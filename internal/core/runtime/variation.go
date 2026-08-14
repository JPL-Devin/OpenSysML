package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// variantReference is the value a name resolving to a variant evaluates to: the
// choice itself, which a variation is bound to and compared with.
func variantReference(sym *symbols.Symbol) Value {
	return Value{Kind: ValVariant, Variant: sym}
}

// IsVariationFeature reports whether a feature is a variation point, whose slot
// holds the variant it is bound to rather than an object of itself.
func (ctx *Context) IsVariationFeature(feat *EffectiveFeature) bool {
	return feat != nil && ctx.model.IsVariationFeature(feat.Symbol)
}

// variantSegment reads a variant named through the variation feature it belongs
// to, as in `ring.nesting::nestingTrue`, returning it and the segments left.
func (ec *EvalContext) variantSegment(feat *EffectiveFeature, rest []ast.NameSegment) (*symbols.Symbol, []ast.NameSegment, bool) {
	if len(rest) == 0 || feat == nil || !ec.ctx.model.IsVariationFeature(feat.Symbol) {
		return nil, nil, false
	}
	variant, ok := ec.ctx.model.VariantOf(feat.Symbol, rest[0].Text)
	if !ok {
		return nil, nil, false
	}
	return variant, rest[1:], true
}

// variantSummary names the variants a variation offers, for a report about a
// selection that is not one of them.
func (ctx *Context) variantSummary(variation *symbols.Symbol) string {
	variants := ctx.model.VariantsOf(variation)
	if len(variants) == 0 {
		return "it declares no variants"
	}
	names := make([]string, 0, len(variants))
	for _, v := range variants {
		names = append(names, v.Name)
	}
	return "variants: " + strings.Join(names, ", ")
}

// bindVariation resolves what a variation feature is bound to into the value its
// slot holds: the selected variant's value, or an object of it (SysML v2 §7.20).
// Anything that is not one of the feature's variants is reported.
func (ctx *Context) bindVariation(feat *EffectiveFeature, selection Value) (Value, error) {
	name := feat.Name
	switch selection.Kind {
	case ValSequence:
		return ctx.bindOneVariant(feat, selection.Sequence.Elements())
	case ValSet:
		return ctx.bindOneVariant(feat, selection.Set.Elements())
	case ValVariant:
	default:
		if ctx.model.IsVariationFeature(feat.Symbol) {
			return Value{}, fmt.Errorf("%w: variation %s is bound to a %s", ErrNotAVariant, name, selection.Kind)
		}
		return selection, nil
	}

	variant := selection.Variant
	if !ctx.model.SelectsVariantOf(feat.Symbol, variant) {
		return Value{}, fmt.Errorf("%w: %s is not a variant of %s (%s)",
			ErrNotAVariant, variant.Name, name, ctx.variantSummary(feat.Symbol))
	}
	return ctx.variantValue(variant)
}

// bindOneVariant binds a variation bound to a collection: exactly one variant
// may be selected, so selecting several, or anything that is not a variant, is
// reported rather than resolved.
func (ctx *Context) bindOneVariant(feat *EffectiveFeature, elements []Value) (Value, error) {
	var selected []Value
	for _, el := range elements {
		if el.Kind != ValVariant {
			return Value{}, fmt.Errorf("%w: variation %s is bound to a collection holding a %s (%s)",
				ErrNotAVariant, feat.Name, el.Kind, ctx.variantSummary(feat.Symbol))
		}
		selected = append(selected, el)
	}
	switch {
	case len(selected) > 1:
		names := make([]string, 0, len(selected))
		for _, el := range selected {
			names = append(names, el.Variant.Name)
		}
		return Value{}, fmt.Errorf("%w: variation %s selects %d variants (%s)",
			ErrMultipleVariants, feat.Name, len(selected), strings.Join(names, ", "))
	case len(selected) == 1:
		return ctx.bindVariation(feat, selected[0])
	default:
		return Value{}, fmt.Errorf("%w: variation %s is bound to a collection naming no variant (%s)",
			ErrNotAVariant, feat.Name, ctx.variantSummary(feat.Symbol))
	}
}

// variantValue materializes a selected variant: the value it declares, or an
// object of it carrying its nested values and connections.
func (ctx *Context) variantValue(variant *symbols.Symbol) (Value, error) {
	if value := semantics.VariantValue(variant); value != nil {
		ec := NewEvalContext(ctx, declScope(variant))
		val, err := ec.Eval(value)
		if err != nil {
			return Value{}, fmt.Errorf("variant %s: %w", variant.Name, err)
		}
		return val, nil
	}
	// One object per variant, so repeated reads of a selection answer with the
	// same object instead of allocating one per read.
	inst, err := ctx.occurrenceOf(variant)
	if err != nil {
		return Value{}, fmt.Errorf("variant %s: %w", variant.Name, err)
	}
	return Value{Kind: ValVariant, Variant: variant, Instance: inst.ID}, nil
}

// variantAsValue resolves a variant to the value it declares, so comparing a
// value with a variant compares what the variant stands for. A variant
// declaring no value is identified by the selection itself.
func (ctx *Context) variantAsValue(v Value) (Value, error) {
	if v.Kind != ValVariant || v.Variant == nil {
		return v, nil
	}
	if semantics.VariantValue(v.Variant) == nil {
		return v, nil
	}
	return ctx.variantValue(v.Variant)
}
