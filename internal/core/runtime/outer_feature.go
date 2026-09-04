package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// outerFeatureValue reads the feature a name resolved to from the nearest object
// in the bound object's ownership chain that carries it. A nested usage naming a
// feature of an enclosing type (`Rectangle::length`, or a sibling `e1`) reads the
// enclosing object's value of it, redefinitions included (KerML 1.0 §7.4.8.1:
// a feature reference denotes the feature relative to the containing object).
func (ec *EvalContext) outerFeatureValue(sym *symbols.Symbol) (Value, bool, error) {
	if sym == nil || !semantics.IsShapeFeature(sym) {
		return Value{}, false, nil
	}
	for inst := ec.self; inst != nil; inst, _ = inst.Owner() {
		name, ok := ec.ctx.featureDenoting(inst, sym)
		if !ok {
			continue
		}
		fv, err := inst.GetFeatureValue(ec.ctx, name)
		if err != nil {
			return Value{}, true, err
		}
		val, err := fv.ReadValue(name)
		return val, true, err
	}
	return Value{}, false, nil
}

// featureDenoting names the feature value of inst that sym denotes: the one whose
// feature is sym or redefines it. A value whose own default is being evaluated is
// skipped, so its expression naming sym reads an outer object's value instead.
func (ctx *Context) featureDenoting(inst *Instance, sym *symbols.Symbol) (string, bool) {
	for _, typ := range inst.types() {
		name, ok := ctx.denotedFeature(typ, sym)
		if !ok {
			continue
		}
		if _, ok := inst.FeatureValues[name]; !ok ||
			ctx.derivingFeatureValues[featureValueRef{instance: inst.ID, feature: name}] {
			continue
		}
		return name, true
	}
	return "", false
}

// denotedFeature names the feature of typ that sym denotes on an object of typ.
func (ctx *Context) denotedFeature(typ, sym *symbols.Symbol) (string, bool) {
	index, ok := ctx.denotedFeatures[typ]
	if !ok {
		index = make(map[*symbols.Symbol]string)
		for _, feat := range ctx.FeaturesOf(typ) {
			if feat.Symbol == nil {
				continue
			}
			if _, seen := index[feat.Symbol]; !seen {
				index[feat.Symbol] = feat.Name
			}
			for _, redefined := range ctx.redefinedFeatures(feat.Symbol, typ) {
				if _, seen := index[redefined]; !seen {
					index[redefined] = feat.Name
				}
			}
		}
		ctx.denotedFeatures[typ] = index
	}
	name, ok := index[sym]
	return name, ok
}

// types answers the type an object was created as followed by the classifiers
// bindings have since added to it.
func (inst *Instance) types() []*symbols.Symbol {
	if inst.Type == nil {
		return inst.classifiers
	}
	return append([]*symbols.Symbol{inst.Type}, inst.classifiers...)
}
