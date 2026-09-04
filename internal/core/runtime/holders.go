package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// holdingFeatures names the features of typ whose stated value lists the named
// feature, so their types classify its objects (KerML 1.0 §7.3.4.1); memoized per type.
func (ctx *Context) holdingFeatures(typ *symbols.Symbol, name string) []string {
	index, ok := ctx.holders[typ]
	if !ok {
		index = make(map[string][]string)
		for _, feat := range ctx.FeaturesOf(typ) {
			if feat.DefaultValue == nil || !ctx.valueBinds(&feat) {
				continue
			}
			for _, held := range ctx.listedFeatures(typ, &feat) {
				if held != feat.Name {
					index[held] = append(index[held], feat.Name)
				}
			}
		}
		ctx.holders[typ] = index
	}
	return index[name]
}

// listedFeatures names the features of typ that feat's value lists as its
// elements: the names of a sequence, or one name standing alone.
func (ctx *Context) listedFeatures(typ *symbols.Symbol, feat *EffectiveFeature) []string {
	scope := feat.DefaultScope()
	if scope == nil {
		return nil
	}
	var names []string
	var visit func(n ast.Node)
	visit = func(n ast.Node) {
		switch n := n.(type) {
		case *ast.SequenceExpr:
			for _, el := range n.Elements {
				visit(el)
			}
		case *ast.FeatureReference:
			if sym := ctx.referencedSymbol(scope, n.Name); sym != nil {
				if name, ok := ctx.denotedFeature(typ, sym); ok {
					names = append(names, name)
				}
			}
		}
	}
	visit(feat.DefaultValue)
	return names
}

// referencedSymbol resolves a name the way the evaluator does: a single part by
// lookup in the scope, more through the qualified-name reader.
func (ctx *Context) referencedSymbol(scope *symbols.Scope, qn *ast.QualifiedName) *symbols.Symbol {
	if qn == nil || len(qn.Parts) == 0 {
		return nil
	}
	if len(qn.Parts) == 1 && !qn.Global {
		if sym, ok := ctx.resolver.LookupName(scope, qn.Parts[0].Text); ok {
			return sym
		}
		return nil
	}
	sym, _ := ctx.resolver.ReadQualified(scope, qn).Symbol()
	return sym
}

// materializeHolders reads the features of inst whose value lists the named feature before its
// objects are made, so every holding feature classifies them whichever is read first.
func (ctx *Context) materializeHolders(inst *Instance, name string) error {
	for _, typ := range inst.types() {
		for _, holder := range ctx.holdingFeatures(typ, name) {
			fv, ok := inst.FeatureValues[holder]
			if !ok || fv.Materialized ||
				ctx.derivingFeatureValues[featureValueRef{instance: inst.ID, feature: holder}] {
				continue
			}
			if _, err := inst.GetFeatureValue(ctx, holder); err != nil {
				return fmt.Errorf("feature %s holding %s: %w", holder, name, err)
			}
		}
	}
	return nil
}
