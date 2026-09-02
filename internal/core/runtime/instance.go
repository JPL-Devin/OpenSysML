package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxMaterializedLowerBound bounds the anonymous objects a collection feature value is
// filled with: a lower bound past it is a model that cannot be materialized
// rather than a run that is merely slow.
const maxMaterializedLowerBound int64 = 1000

// maxBehaviorBindingDepth bounds the chain of names an `exhibit`/`perform`
// declaration is followed through to the element stating the body, so a binding
// that names itself is reported rather than followed forever.
const maxBehaviorBindingDepth = 32

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

	// behaviors are the executions of the behaviors the object's type exhibits or
	// performs, in declaration order, each bound to this object's identity.
	behaviors []*ObjectBehavior

	// owner is the object holding this one as the feature value named ownerFeature,
	// nil for an object no other holds. A nested object reaches its siblings
	// through it.
	owner        *Instance
	ownerFeature string
}

// Owner answers the object holding this one and the feature of it that does, or
// nil and "" for an object no other holds.
func (inst *Instance) Owner() (*Instance, string) {
	return inst.owner, inst.ownerFeature
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
	Feature        *EffectiveFeature
	Value          Value // scalar feature value (multiplicity [1])
	Values         Value // collection feature value (Sequence or Set)
	Materialized   bool  // lazy flag: has this feature value been instantiated?
	Written        bool  // a run assigned this value, so no default derives it again
	BindingDerived bool  // value came from binding propagation rather than a write
}

// HeldValue is the value the feature value reads as: its collection when the feature is
// multi-valued, otherwise its scalar.
func (s *FeatureValue) HeldValue() Value {
	if s.Values.Kind != ValInvalid {
		return s.Values
	}
	return s.Value
}

// ReadValue is what the feature value reads as in an expression: what it holds, or the
// empty sequence when it holds nothing and its multiplicity admits that. A required
// feature holding nothing is uninitialized.
func (s *FeatureValue) ReadValue(name string) (Value, error) {
	value := s.HeldValue()
	if value.Kind != ValInvalid {
		return value, nil
	}
	if lower := s.Feature.Multiplicity.Lower; lower.Known && lower.Value == 0 {
		return sequenceOf(nil), nil
	}
	return Value{}, fmt.Errorf("%w: %s", ErrUninitializedFeatureValue, name)
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
// leaves composite features lazy, then starts the behaviors the type exhibits or
// performs and runs them to quiescence. Returns the instance or an error.
//
// Each call materializes a distinct object with an identity and behaviors of its
// own; occurrenceOf is the path that reads one object of a usage twice.
func (ctx *Context) Instantiate(sym *symbols.Symbol) (*Instance, error) {
	return ctx.instantiateAs(sym, 0)
}

// instantiateAs materializes an object under the given identity, falling back to
// the next one this context hands out when that identity is none or taken here.
func (ctx *Context) instantiateAs(sym *symbols.Symbol, id int64) (*Instance, error) {
	return ctx.instantiateOwnedBy(sym, id, nil, "")
}

// instantiateOwnedBy materializes an object held by owner as the feature value
// named feature. The owner is recorded before any behavior starts, so a behavior
// addressing a sibling reaches the object its owner holds rather than a second
// one.
func (ctx *Context) instantiateOwnedBy(sym *symbols.Symbol, id int64, owner *Instance, feature string) (*Instance, error) {
	// A creation that fails leaves none of the objects it reached behind, however
	// deeply nested or however a behavior of it addressed them.
	mark := len(ctx.created)
	inst, err := ctx.materializeOwnedBy(sym, id, owner, feature)
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		return nil, err
	}
	return inst, nil
}

// materializeOwnedBy materializes an object held by owner as the feature value
// named feature, without starting its behaviors: a holder records the object it
// holds before those behaviors run, so one addressing it back through its holder
// reaches this object rather than materializing a second.
func (ctx *Context) materializeOwnedBy(sym *symbols.Symbol, id int64, owner *Instance, feature string) (*Instance, error) {
	inst, err := ctx.materialize(sym, id)
	if err != nil {
		return nil, err
	}
	inst.owner, inst.ownerFeature = owner, feature
	return inst, nil
}

// materialize builds the object and its feature values and registers it, before
// any behavior of it starts: an entry action reads the object's declared
// defaults, and a behavior can already reach the object it belongs to.
func (ctx *Context) materialize(sym *symbols.Symbol, id int64) (*Instance, error) {
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

		// Fold constant defaults the feature admits eagerly; a default that is not constant
		// may read sibling feature values, so GetFeatureValue evaluates (and reports) it.
		if ctx.valueBinds(feat) && feat.Scalar() && !ctx.model.IsVariationFeature(feat.Symbol) &&
			ctx.restatedInValuedBody(feat) == "" {
			if semVal, ok := ctx.model.Eval(feat.DefaultValue); ok {
				val := Value{Kind: ValConst, Const: semVal}
				if ctx.checkDefault(inst, fv, feat.Name, val) == nil {
					fv.Value = val
					fv.Materialized = true
				}
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

	if ctx.trace != nil {
		ctx.trace.RecordObjectMaterialized(symbolText(sym), inst.ID)
	}

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
	// The occurrence is recorded before its behaviors start, so a behavior that
	// reaches the usage it belongs to reads this object rather than a second one.
	mark := len(ctx.created)
	inst, err := ctx.materialize(sym, 0)
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	ctx.occurrences[sym] = inst.ID
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		return nil, err
	}
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

// namesOneObject reports whether a usage denotes one object of its own — which
// a name evaluates to and a feature chain reads members from — rather than a
// value: an occurrence, or a structured value whose own features carry it.
func (ctx *Context) namesOneObject(sym *symbols.Symbol) bool {
	if sym == nil || !ctx.occursOnce(sym) {
		return false
	}
	// A variation classifies its variants abstractly, so it is no object of
	// itself: it holds nothing until it is bound to one.
	if ctx.model.IsVariationFeature(sym) {
		return false
	}
	return isOccurrenceUsage(sym) || ctx.namesStructuredValue(sym)
}

// namesStructuredValue reports whether a usage carrying no value of its own is
// typed by a structured value — a non-scalar `attribute def` with features — so
// it holds those features rather than one scalar value.
func (ctx *Context) namesStructuredValue(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value != nil || usage.Kind != ast.UsageAttribute {
		return false
	}
	typ := ctx.extractType(sym)
	if typ == nil || ctx.model.PrimTypeOf(typ) != semantics.PrimUnknown {
		return false
	}
	return len(ctx.FeaturesOf(sym)) > 0
}

// occursOnce reports whether a usage names at most one occurrence; several
// occurrences are a collection rather than one object to read features from.
func (ctx *Context) occursOnce(sym *symbols.Symbol) bool {
	mult, _ := ctx.extractMultiplicity(sym)
	return !mult.Upper.Infinite && mult.Upper.Value <= 1
}

// checkDefault reports a value the feature does not admit: a count outside its
// multiplicity (1..1 when none is declared) or an element outside its type.
func (ctx *Context) checkDefault(inst *Instance, fv *FeatureValue, name string, val Value) error {
	what := fmt.Sprintf("feature value %s.%s", inst.Type.Name, name)
	if msg := fv.Feature.Multiplicity.CountViolation(elementCount(&val)); msg != "" {
		return fmt.Errorf("%s: %w: %s", what, ErrMultiplicityViolation, msg)
	}
	return ctx.checkWriteType(fv.Feature.DeclScope(), what, fv.Feature.Type, val)
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

// SetFeatureValue writes a value to the named feature value of the object, which is how a
// behavior the object performs updates the object's own state. The value must
// conform to the multiplicity governing the feature; a feature the object does
// not have is reported rather than added.
func (inst *Instance) SetFeatureValue(ctx *Context, name string, value Value) error {
	fv, ok := inst.FeatureValues[name]
	if !ok {
		return fmt.Errorf("feature %q not found in instance %d (type %s)", name, inst.ID, inst.Type.Name)
	}
	// Checked before the write, so a value the feature does not admit leaves it
	// holding what it held.
	if err := ctx.checkDefault(inst, fv, name, value); err != nil {
		return err
	}
	if fv.Feature.Scalar() {
		fv.Value = value
		fv.Values = Value{}
	} else {
		// A multi-valued feature holds a collection however it was written, so a
		// single value written to one is that collection's one element.
		if value.Kind != ValSequence && value.Kind != ValSet {
			elements := elementsOf(value)
			if err := ctx.chargeElements(int64(len(elements))); err != nil {
				return err
			}
			value = sequenceOf(elements)
		}
		fv.Values = value
		fv.Value = Value{}
	}
	fv.Materialized, fv.Written = true, true
	fv.BindingDerived = false
	return nil
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
	// feature's multiplicity and type.
	if ctx.valueBinds(fv.Feature) {
		val, err := ctx.evalFeatureValueDefault(inst, fv, name)
		if err != nil {
			return nil, err
		}
		if err := ctx.checkDefault(inst, fv, name, val); err != nil {
			return nil, err
		}
		if fv.Feature.Scalar() {
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

		// An abstract feature has no values of its own (KerML 1.0 §7.3.3.1), and
		// an optional one demands none: each holds only what the features
		// subsetting it hold, as a collection fills only to its lower bound.
		if symbols.IsAbstract(fv.Feature.Symbol) || (mult.Lower.Value == 0 && fv.Feature.Scalar()) {
			return inst.holdContributions(ctx, fv, name)
		}

		if !mult.Upper.Infinite && mult.Upper.Value == 1 {
			// Scalar: instantiate one, held by this feature before its behaviors
			// start, so one addressing it back reads the object held here.
			mark := len(ctx.created)
			childInst, err := ctx.materializeOwnedBy(composite, 0, inst, name)
			if err != nil {
				ctx.abandonInstancesSince(mark)
				return nil, err
			}
			fv.Value = Value{Kind: ValInstance, Instance: childInst.ID}
			fv.Materialized = true
			if err := ctx.startClassifierBehaviors(childInst, mark); err != nil {
				return nil, err
			}
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
			// The whole collection is held before any of its objects starts, so a
			// behavior reading the feature back reads the objects held in it.
			mark := len(ctx.created)
			children := make([]*Instance, 0, count)
			for i := 0; i < count; i++ {
				childInst, err := ctx.materializeOwnedBy(composite, 0, inst, name)
				if err != nil {
					ctx.abandonInstancesSince(mark)
					return nil, err
				}
				seq.Append(Value{Kind: ValInstance, Instance: childInst.ID})
				children = append(children, childInst)
			}
			fv.Values = NewSequenceValue(seq)
			fv.Materialized = true
			if err := ctx.startClassifierBehaviorsOf(children, mark); err != nil {
				return nil, err
			}
		}
		fv.Materialized = true
	}

	return fv, nil
}

// holdContributions fills an abstract feature with the values the features
// subsetting it contribute, checked against the multiplicity governing it.
func (inst *Instance) holdContributions(ctx *Context, fv *FeatureValue, name string) (*FeatureValue, error) {
	contributed, err := ctx.subsettingContributions(inst, name)
	if err != nil {
		return nil, err
	}
	if fv.Feature.Scalar() {
		if len(contributed) > 1 {
			return nil, fmt.Errorf("feature value %s.%s: %w: %s", inst.Type.Name, name, ErrMultiplicityViolation,
				fv.Feature.Multiplicity.CountViolation(int64(len(contributed))))
		}
		if len(contributed) == 1 {
			fv.Value = contributed[0]
		}
	} else {
		fv.Values = sequenceOf(contributed)
	}
	fv.Materialized = true
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
