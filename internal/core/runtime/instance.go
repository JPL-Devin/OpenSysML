package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxMaterializedLowerBound bounds the anonymous objects a collection feature value is
// filled with: a lower bound past it is a model that cannot be materialized
// rather than a run that is merely slow.
const maxMaterializedLowerBound int64 = 1000

// Instance is a runtime-materialized object (Tier 2).
type Instance struct {
	ID            int64                    // unique identity
	Type          *symbols.Symbol          // the def/usage symbol this instantiates
	FeatureValues map[string]*FeatureValue // feature name → feature value
	// Ends are the ends of the connector this object materializes, in declaration
	// order, and nil for an object that is no connector. A named end also reads
	// through the feature value of that name; the order is what an end with no name of its
	// own is identified by.
	Ends []ConnectorEnd

	// anonymous holds the objects the instance's anonymous connectors
	// materialized to, nil until they are asked for. An empty slice means there
	// are none.
	anonymous []int64

	// keptAnonymous holds the identities those objects had before a carry-over, in
	// declaration order, which the ones materialized again here take back.
	keptAnonymous []int64

	// keptConnectors holds, per feature value of a named connector, the identity the object
	// of it had before a carry-over, which the one materialized again takes back.
	keptConnectors map[*FeatureValue]int64
}

// keepConnector remembers the identity the object of a named connector feature value had,
// so the one materialized again against the new declarations keeps it.
func (inst *Instance) keepConnector(fv *FeatureValue, id int64) {
	if inst.keptConnectors == nil {
		inst.keptConnectors = make(map[*FeatureValue]int64)
	}
	inst.keptConnectors[fv] = id
}

// Feature value holds the runtime value(s) for one feature.
type FeatureValue struct {
	Feature      *EffectiveFeature
	Value        Value // scalar feature value (multiplicity [1])
	Values       Value // collection feature value (Sequence or Set)
	Materialized bool  // lazy flag: has this feature value been instantiated?
}

// HeldValue is the value the feature value reads as: its collection when the feature is
// multi-valued, otherwise its scalar.
func (s *FeatureValue) HeldValue() Value {
	if s.Values.Kind != ValInvalid {
		return s.Values
	}
	return s.Value
}

// UnsetText is how every surface spells a feature value that holds no value: a valueless
// feature of a value type, whose instances are values rather than objects.
const UnsetText = "<unset>"

// HoldsNoValue reports whether a value is an object materialized for a valueless
// feature of a value type. Such an object has no feature that could hold a value
// and is no value itself (KerML: a DataType classifies values), so it reads as
// unset rather than as an object.
func (ctx *Context) HoldsNoValue(val Value) bool {
	if val.Kind != ValInstance {
		return false
	}
	inst, ok := ctx.instances[val.Instance]
	return ok && len(inst.FeatureValues) == 0 && isValueTypeSymbol(inst.Type)
}

// isValueTypeSymbol reports whether sym declares a value type: a `datatype` or
// `attribute def` (KerML DataType) or an enumeration, as distinct from a class
// whose instances are objects. Mirrors passes.isDataTypeDefKind.
func isValueTypeSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolAttributeDef, symbols.SymbolEnumerationDef,
		symbols.SymbolAttributeUsage, symbols.SymbolEnumerationUsage:
		return true
	default:
		return false
	}
}

// Instantiate materializes an instance of the given usage/definition symbol.
// Allocates ID, creates feature values per FeaturesOf(sym), evaluates default values,
// leaves composite features lazy. Returns the instance or an error.
func (ctx *Context) Instantiate(sym *symbols.Symbol) (*Instance, error) {
	return ctx.instantiateAs(sym, 0)
}

// instantiateAs materializes an object under the given identity, falling back to
// the next one this context hands out when that identity is none or taken here.
func (ctx *Context) instantiateAs(sym *symbols.Symbol, id int64) (*Instance, error) {
	defer ctx.beginRun()()

	// Check step limit (I3)
	if err := ctx.incrementStep(); err != nil {
		return nil, err
	}

	if _, taken := ctx.instances[id]; taken || id <= 0 {
		id = ctx.allocateID()
	}
	ctx.ids.atLeast(id + 1)

	// Create instance
	inst := &Instance{
		ID:            id,
		Type:          sym,
		FeatureValues: make(map[string]*FeatureValue),
	}

	// Get effective features
	features := ctx.FeaturesOf(sym)

	// Create feature value for each feature
	for i := range features {
		feat := &features[i]
		fv := &FeatureValue{
			Feature:      feat,
			Materialized: false,
		}

		// Fold constant defaults eagerly. A default that is not constant may read
		// sibling feature values of this very instance, so it is left to GetFeatureValue, which
		// evaluates it against the finished instance.
		if ctx.valueBinds(feat) && isScalarFeature(feat) && !ctx.model.IsVariationFeature(feat.Symbol) &&
			ctx.restatedInValuedBody(feat) == "" {
			if semVal, ok := ctx.model.Eval(feat.DefaultValue); ok {
				fv.Value = Value{Kind: ValConst, Const: semVal}
				fv.Materialized = true
			}
		}

		inst.FeatureValues[feat.Name] = fv
	}

	// A redefining feature declares the feature it redefines again, so the two
	// names read one feature value.
	if err := ctx.aliasRedefinedFeatureValues(inst); err != nil {
		return nil, err
	}

	// Register instance
	ctx.registerInstance(inst)

	return inst, nil
}

// occurrenceOf returns the object a usage denotes, materializing it once: a part
// declared in a package names one occurrence, so reading its features twice
// reads the same object.
func (ctx *Context) occurrenceOf(sym *symbols.Symbol) (*Instance, error) {
	if id, ok := ctx.occurrences[sym]; ok {
		if inst, ok := ctx.instances[id]; ok {
			return inst, nil
		}
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return nil, err
	}
	ctx.occurrences[sym] = inst.ID
	return inst, nil
}

// isOccurrenceUsage reports whether sym declares a usage that is an occurrence:
// a part, item or individual, which is a thing with features rather than a
// value, so a chain through it reads the features of that thing.
func isOccurrenceUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value != nil {
		return false
	}
	switch usage.Kind {
	case ast.UsagePart, ast.UsageItem, ast.UsageOccurrence, ast.UsageIndividual:
		return true
	default:
		return false
	}
}

// occursOnce reports whether a usage names at most one occurrence; several
// occurrences are a collection rather than one object to read features from.
func (ctx *Context) occursOnce(sym *symbols.Symbol) bool {
	mult, _ := ctx.extractMultiplicity(sym)
	return !mult.Upper.Infinite && mult.Upper.Value <= 1
}

// isScalarFeature reports whether a feature holds at most one value. An
// unbounded upper bound carries Value 0, so the infinite flag has to be tested
// separately.
func isScalarFeature(feat *EffectiveFeature) bool {
	return !feat.Multiplicity.Upper.Infinite && feat.Multiplicity.Upper.Value <= 1
}

// checkDefaultCount reports a default whose element count does not conform to
// the multiplicity governing the feature, which is the assumed 1..1 for a
// feature that declares none. A conforming default is merged as written; a
// non-conforming one is neither broadcast nor padded, since that would invent
// values the model does not state. The count of an expression's result is only
// known here, so this is where such a default is reported; a count the type tier
// can see statically is reported there (passes.checkValueCount).
func (ctx *Context) checkDefaultCount(inst *Instance, fv *FeatureValue, name string, val Value) error {
	count := int64(len(elementsOf(val)))
	if msg := fv.Feature.Multiplicity.CountViolation(count); msg != "" {
		return fmt.Errorf("feature value %s.%s: %w: %s", inst.Type.Name, name, ErrMultiplicityViolation, msg)
	}
	return nil
}

// GetFeatureValue retrieves the feature value for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet. A feature value that could
// not be materialized is marked as such — it unwraps to ErrFeatureValueMaterialization —
// so a caller can tell it from any other failure to evaluate, whatever the
// expression it surfaced through.
func (inst *Instance) GetFeatureValue(ctx *Context, name string) (*FeatureValue, error) {
	if _, ok := inst.FeatureValues[name]; !ok {
		// Naming no feature value of the object is no materialization of one.
		return nil, fmt.Errorf("feature %q not found in instance %d (type %s)", name, inst.ID, inst.Type.Name)
	}
	fv, err := inst.materializeFeatureValue(ctx, name)
	if err != nil {
		return nil, &FeatureValueError{Err: err}
	}
	return fv, nil
}

// materializeFeatureValue is GetFeatureValue's materialization: the feature value's value, evaluated and
// checked against the multiplicity governing its feature the first time it is read.
func (inst *Instance) materializeFeatureValue(ctx *Context, name string) (*FeatureValue, error) {
	defer ctx.beginRun()()

	fv := inst.FeatureValues[name]

	if val, found, err := ctx.resolveBindingValue(inst, name); err != nil {
		return nil, err
	} else if found {
		if err := ctx.assignBindingValue(inst, fv, name, val); err != nil {
			return nil, err
		}
		return fv, nil
	}

	// If already materialized, return
	if fv.Materialized {
		return fv, nil
	}

	return inst.materializeFeatureValueIntrinsic(ctx, name)
}

// materializeFeatureValueIntrinsic evaluates a feature without following
// binding connectors; binding resolution calls it to inspect an endpoint.
func (inst *Instance) materializeFeatureValueIntrinsic(ctx *Context, name string) (*FeatureValue, error) {
	fv := inst.FeatureValues[name]

	// A variation holds the variant it was bound to, and nothing until it is
	// bound: it classifies its variants abstractly, so it is no object of itself.
	if ctx.model.IsVariationFeature(fv.Feature.Symbol) {
		if fv.Feature.DefaultValue == nil {
			return nil, fmt.Errorf("%w: %s.%s", ErrVariationUnselected, inst.Type.Name, name)
		}
		val, err := ctx.evalFeatureValueDefault(inst, fv, name)
		if err != nil {
			return nil, err
		}
		bound, err := ctx.bindVariation(fv.Feature, val, inst.ID)
		if err != nil {
			return nil, fmt.Errorf("feature value %s.%s: %w", inst.Type.Name, name, err)
		}
		fv.Value = bound
		fv.Materialized = true
		return fv, nil
	}

	// A bound value supplies the feature's own features, so a body restating one
	// of them states two values for it.
	if restated := ctx.restatedInValuedBody(fv.Feature); restated != "" {
		return nil, fmt.Errorf("feature value %s.%s: %w: %s", inst.Type.Name, name, ErrValuedFeatureRestated, restated)
	}

	// A default that did not constant-fold is a derived value: evaluate it
	// against this instance, so that it sees the sibling feature values it refers to.
	// The feature holds what the default states, once that conforms to the
	// feature's multiplicity.
	if ctx.valueBinds(fv.Feature) {
		val, err := ctx.evalFeatureValueDefault(inst, fv, name)
		if err != nil {
			return nil, err
		}
		if err := ctx.checkDefaultCount(inst, fv, name, val); err != nil {
			return nil, err
		}
		if isScalarFeature(fv.Feature) {
			fv.Value = val
		} else {
			// A multi-valued feature holds a collection, so a single value
			// stated as its default is that collection's one element, and a
			// default that is no value at all holds nothing: the elements
			// stored are the ones counted above.
			if val.Kind != ValSequence && val.Kind != ValSet {
				elements := elementsOf(val)
				if err := ctx.chargeElements(int64(len(elements))); err != nil {
					return nil, err
				}
				val = sequenceOf(elements)
			}
			fv.Values = val
		}
		fv.Materialized = true
		return fv, nil
	}

	// A connector holds the features it connects at its ends rather than objects
	// of its own, so it is materialized from what the `connect` clause names.
	if ctx.model.IsConnectorUsage(fv.Feature.Symbol) {
		if err := ctx.materializeConnectorFeatureValue(inst, fv, name); err != nil {
			return nil, err
		}
		return fv, nil
	}

	// Lazy instantiation: a composite feature holds objects of its own.
	if composite := ctx.CompositeTypeOf(fv.Feature); composite != nil {
		// Check multiplicity (C2 + C1)
		mult := fv.Feature.Multiplicity
		if !mult.Upper.Known || !mult.Lower.Known {
			return nil, fmt.Errorf("cannot materialize feature %q with unknown multiplicity", name)
		}

		if !mult.Upper.Infinite && mult.Upper.Value == 1 {
			// Scalar: instantiate one
			childInst, err := ctx.Instantiate(composite)
			if err != nil {
				return nil, err
			}
			fv.Value = Value{Kind: ValInstance, Instance: childInst.ID}
		} else {
			// Guard against infinite/huge lower bound (C3)
			if mult.Lower.Infinite || mult.Lower.Value > maxMaterializedLowerBound {
				return nil, fmt.Errorf("%w: lower bound too large or infinite for feature %q", ErrMultiplicityViolation, name)
			}

			// A feature subsetting this one holds values this one holds, so the
			// objects those features name are members of this collection;
			// anonymous objects make up the rest of the lower bound.
			contributed, err := ctx.subsettingContributions(inst, name)
			if err != nil {
				return nil, err
			}

			count := int(mult.Lower.Value) - len(contributed)
			if count < 0 {
				count = 0
			}

			// Determine collection type (Sequence vs Set)
			if err := ctx.chargeElements(int64(count)); err != nil {
				return nil, err
			}
			seq := NewSequence()
			for _, val := range contributed {
				seq.Append(val)
			}
			for i := 0; i < count; i++ {
				childInst, err := ctx.Instantiate(composite)
				if err != nil {
					return nil, err
				}
				seq.Append(Value{Kind: ValInstance, Instance: childInst.ID})
			}
			fv.Values = Value{Kind: ValSequence, Sequence: seq}
		}
		fv.Materialized = true
	}

	return fv, nil
}

// CompositeTypeOf returns what a feature is materialized from, or nil for one
// that holds a value rather than an object — a default that binds takes precedence
// over instantiation, as in GetFeatureValue above. A usage with features of its own is
// instantiated as itself, so its body governs and an untyped nested part
// materializes at all. Answering costs no allocation, so a caller walking an
// object graph can decide whether to descend before descending.
func (ctx *Context) CompositeTypeOf(feat *EffectiveFeature) *symbols.Symbol {
	if ctx.valueBinds(feat) {
		return nil
	}
	// A variation is materialized from the variant it is bound to, never from
	// itself: it is an abstract classifier of its variants.
	if ctx.model.IsVariationFeature(feat.Symbol) {
		return nil
	}
	if feat.Symbol != nil && declaresFeatures(feat.Symbol) {
		return feat.Symbol
	}
	return feat.Type
}

// declaresFeatures reports whether a usage's own body restates or adds features,
// which the object it materializes has to carry.
func declaresFeatures(sym *symbols.Symbol) bool {
	for _, member := range declMembers(sym.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		if usage.Ident.Name != "" || usage.Ident.ShortName != "" || len(usage.Relationships) > 0 {
			return true
		}
	}
	return false
}

// valueBinds reports whether the value bound to a feature governs it: a redefining
// declaration's own value body governs over one the redefined declaration wrote,
// being the more specific declaration of the feature (KerML 1.0 §7.3.4.5).
func (ctx *Context) valueBinds(feat *EffectiveFeature) bool {
	if feat == nil || feat.DefaultValue == nil {
		return false
	}
	return !ctx.bodyGovernsInheritedValue(feat)
}

// bodyGovernsInheritedValue reports whether a feature's own body values what the value
// it inherits from the declaration it redefines would supply, superseding that value.
func (ctx *Context) bodyGovernsInheritedValue(feat *EffectiveFeature) bool {
	if feat.Symbol == nil || feat.DefaultDecl == nil || feat.DefaultDecl == feat.Symbol {
		return false
	}
	return ctx.restatedValueInBody(feat.Symbol, feat.Type) != ""
}

// restatedInValuedBody returns the name of a feature valued again in the body of the
// declaration that binds a value to it — two values, neither more specific — or ""
// when there is none. A body over an inherited value governs instead, above.
func (ctx *Context) restatedInValuedBody(feat *EffectiveFeature) string {
	if feat.Symbol == nil {
		return ""
	}
	decl, ok := feat.Symbol.Decl.(*ast.Usage)
	if !ok || decl.Value == nil {
		return ""
	}
	return ctx.restatedValueInBody(feat.Symbol, feat.Type)
}

// restatedValueInBody returns the name of a feature the body of sym values
// again — restating it with `:>>`/`:>`, or re-declaring a feature typ carries —
// or "" when the body values none of them.
func (ctx *Context) restatedValueInBody(sym, typ *symbols.Symbol) string {
	inherited := make(map[string]bool)
	for _, f := range ctx.FeaturesOf(typ) {
		inherited[f.Name] = true
	}
	for _, member := range declMembers(sym.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok || !valuesAFeature(usage) {
			continue
		}
		if name := restatedFeatureName(usage); name != "" {
			return name
		}
		if inherited[usage.Ident.Name] {
			return usage.Ident.Name
		}
	}
	return ""
}

// valuesAFeature reports whether a usage states a value: its own, or one its
// body states at any depth. A body that only re-declares features states none.
func valuesAFeature(usage *ast.Usage) bool {
	if usage.Value != nil {
		return true
	}
	for _, member := range declMembers(usage) {
		if nested, ok := member.(*ast.Usage); ok && valuesAFeature(nested) {
			return true
		}
	}
	return false
}

// restatedFeatureName returns the name a usage restates with `:>>` or `:>`, or
// "" when it restates nothing.
func restatedFeatureName(usage *ast.Usage) string {
	for _, rel := range usage.Relationships {
		if rel == nil || (rel.Kind != ast.RelRedefines && rel.Kind != ast.RelSubsets) {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok && len(qn.Parts) > 0 {
			return qn.Parts[len(qn.Parts)-1].Text
		}
	}
	return ""
}

// evalFeatureValueDefault evaluates a feature value's default-value expression bound to the
// owning instance. Recursion back through the feature value being computed is reported
// as ErrCyclicFeatureValue rather than recursing until the step budget runs out.
func (ctx *Context) evalFeatureValueDefault(inst *Instance, fv *FeatureValue, name string) (Value, error) {
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.derivingFeatureValues[key] {
		return Value{}, fmt.Errorf("%w: %s.%s", ErrCyclicFeatureValue, inst.Type.Name, name)
	}
	ctx.derivingFeatureValues[key] = true
	defer delete(ctx.derivingFeatureValues, key)

	scope := fv.Feature.DefaultScope()
	if scope == nil {
		scope = inst.Type.OwnerScope
	}
	ec := NewEvalContextIn(ctx, scope, inst)
	defer ec.beginStep()()
	val, err := ec.Eval(fv.Feature.DefaultValue)
	if err != nil {
		return Value{}, fmt.Errorf("feature value %s.%s: %w", inst.Type.Name, name, err)
	}
	return val, nil
}
