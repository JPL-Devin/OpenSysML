package runtime

import (
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// instanceConforms reports whether an object is an instance of typ by its declaration or by
// a feature it was held as a value of (KerML 1.0 §7.3.4.1: a feature's values are instances of its types).
func (ctx *Context) instanceConforms(inst *Instance, typ *symbols.Symbol) bool {
	if ctx.model.Conforms(inst.Type, typ) {
		return true
	}
	for _, c := range inst.classifiers {
		if ctx.model.Conforms(c, typ) {
			return true
		}
	}
	return false
}

// canClassify reports whether an object may be held by a feature typed by typ: it is one
// already, or every type classifying it is comparable with typ, so holding it narrows it.
func (ctx *Context) canClassify(inst *Instance, typ *symbols.Symbol) bool {
	if ctx.instanceConforms(inst, typ) {
		return true
	}
	if !ctx.comparableTypes(inst.Type, typ, map[*symbols.Symbol]bool{}) {
		return false
	}
	for _, c := range inst.classifiers {
		if !ctx.comparableTypes(c, typ, map[*symbols.Symbol]bool{}) {
			return false
		}
	}
	return true
}

// comparableTypes reports whether one of typ and other specializes the other. A
// feature stands for the types it is typed by, implicit base included.
func (ctx *Context) comparableTypes(typ, other *symbols.Symbol, seen map[*symbols.Symbol]bool) bool {
	if typ == nil || other == nil || ctx.model.Conforms(typ, other) || ctx.model.Conforms(other, typ) {
		return true
	}
	if !semantics.IsShapeFeature(typ) || seen[typ] {
		return false
	}
	seen[typ] = true
	for _, super := range ctx.model.DirectSupertypes(typ) {
		if !ctx.comparableTypes(super, other, seen) {
			return false
		}
	}
	return true
}

// classifyHeld classifies every object held as a value of feature by the feature itself,
// so it carries the features its type and body declare; the caller has checked each may be held.
func (ctx *Context) classifyHeld(feature *symbols.Symbol, val Value) error {
	if feature == nil {
		return nil
	}
	for _, el := range elementsOf(val) {
		if el.Kind != ValInstance {
			continue
		}
		inst, ok := ctx.instances[el.Instance]
		if !ok || inst == nil || inst.Type == nil {
			continue
		}
		if err := ctx.classify(inst, feature); err != nil {
			return err
		}
	}
	return nil
}

// classify records typ as a classifier of inst with the features and behaviors it adds;
// a classification that fails or that a probe rolls back leaves the object as it was.
func (ctx *Context) classify(inst *Instance, typ *symbols.Symbol) error {
	if ctx.instanceConforms(inst, typ) {
		return nil
	}
	classifiers, values := inst.classifiers, maps.Clone(inst.FeatureValues)
	var started []*ObjectBehavior
	restore := func() {
		inst.classifiers, inst.FeatureValues = classifiers, values
		ctx.forgetBehaviors(started)
	}
	if ctx.journals > 0 {
		ctx.noteProbeUndo(restore)
	}
	inst.classifiers = append(inst.classifiers, typ)
	carried := make(map[string]bool, len(inst.FeatureValues))
	for name := range inst.FeatureValues {
		carried[name] = true
	}
	features := ctx.FeaturesOf(typ)
	for i := range features {
		feat := &features[i]
		if carried[feat.Name] {
			continue
		}
		inst.FeatureValues[feat.Name] = ctx.newFeatureValue(inst, feat)
	}
	if err := ctx.aliasRedefinedFeatureValuesOf(inst, typ, carried); err != nil {
		restore()
		return err
	}
	running := len(inst.behaviors)
	if err := ctx.startClassifierBehaviors(inst, len(ctx.created)); err != nil {
		restore()
		return err
	}
	started = slices.Clone(inst.behaviors[running:])
	return nil
}
