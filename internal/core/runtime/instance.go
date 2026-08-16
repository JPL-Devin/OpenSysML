package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// maxMaterializedLowerBound bounds the anonymous objects a collection slot is
// filled with: a lower bound past it is a model that cannot be materialized
// rather than a run that is merely slow.
const maxMaterializedLowerBound int64 = 1000

// Instance is a runtime-materialized object (Tier 2).
type Instance struct {
	ID    int64            // unique identity
	Type  *symbols.Symbol  // the def/usage symbol this instantiates
	Slots map[string]*Slot // feature name → slot
	// Ends are the ends of the connector this object materializes, in declaration
	// order, and nil for an object that is no connector. A named end also reads
	// through the slot of that name; the order is what an end with no name of its
	// own is identified by.
	Ends []ConnectorEnd

	// anonymous holds the objects the instance's anonymous connectors
	// materialized to, nil until they are asked for. An empty slice means there
	// are none.
	anonymous []int64

	// keptAnonymous holds the identities those objects had before a carry-over, in
	// declaration order, which the ones materialized again here take back.
	keptAnonymous []int64

	// keptConnectors holds, per slot of a named connector, the identity the object
	// of it had before a carry-over, which the one materialized again takes back.
	keptConnectors map[*Slot]int64
}

// keepConnector remembers the identity the object of a named connector slot had,
// so the one materialized again against the new declarations keeps it.
func (inst *Instance) keepConnector(slot *Slot, id int64) {
	if inst.keptConnectors == nil {
		inst.keptConnectors = make(map[*Slot]int64)
	}
	inst.keptConnectors[slot] = id
}

// Slot holds the runtime value(s) for one feature.
type Slot struct {
	Feature      *EffectiveFeature
	Value        Value // scalar slot (multiplicity [1])
	Values       Value // collection slot (Sequence or Set)
	Materialized bool  // lazy flag: has this slot been instantiated?
}

// HeldValue is the value the slot reads as: its collection when the feature is
// multi-valued, otherwise its scalar.
func (s *Slot) HeldValue() Value {
	if s.Values.Kind != ValInvalid {
		return s.Values
	}
	return s.Value
}

// Instantiate materializes an instance of the given usage/definition symbol.
// Allocates ID, creates slots per FeaturesOf(sym), evaluates default values,
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
		ID:    id,
		Type:  sym,
		Slots: make(map[string]*Slot),
	}

	// Get effective features
	features := ctx.FeaturesOf(sym)

	// Create slot for each feature
	for i := range features {
		feat := &features[i]
		slot := &Slot{
			Feature:      feat,
			Materialized: false,
		}

		// Fold constant defaults eagerly. A default that is not constant may read
		// sibling slots of this very instance, so it is left to GetSlot, which
		// evaluates it against the finished instance.
		if feat.DefaultValue != nil && isScalarFeature(feat) && !ctx.model.IsVariationFeature(feat.Symbol) &&
			ctx.restatedInValuedBody(feat) == "" {
			if semVal, ok := ctx.model.Eval(feat.DefaultValue); ok {
				slot.Value = Value{Kind: ValConst, Const: semVal}
				slot.Materialized = true
			}
		}

		inst.Slots[feat.Name] = slot
	}

	// A redefining feature declares the feature it redefines again, so the two
	// names read one slot.
	if err := ctx.aliasRedefinedSlots(inst); err != nil {
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
// the multiplicity the feature states. A conforming default is merged as
// written; a non-conforming one is neither broadcast nor padded, since that
// would invent values the model does not state. The count of an expression's
// result is only known here, so this is where such a default is reported; a
// count the type tier can see statically is reported there (passes.checkValueCount).
func (ctx *Context) checkDefaultCount(inst *Instance, slot *Slot, name string, val Value) error {
	// 1..1 assumed rather than stated is no declared bound to hold a default to.
	if !slot.Feature.MultiplicityStated {
		return nil
	}
	count := int64(len(elementsOf(val)))
	if msg := slot.Feature.Multiplicity.CountViolation(count); msg != "" {
		return fmt.Errorf("slot %s.%s: %w: %s", inst.Type.Name, name, ErrMultiplicityViolation, msg)
	}
	return nil
}

// GetSlot retrieves the slot for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet.
func (inst *Instance) GetSlot(ctx *Context, name string) (*Slot, error) {
	defer ctx.beginRun()()

	slot, ok := inst.Slots[name]
	if !ok {
		return nil, fmt.Errorf("slot %q not found in instance %d (type %s)", name, inst.ID, inst.Type.Name)
	}

	// If already materialized, return
	if slot.Materialized {
		return slot, nil
	}

	// A variation holds the variant it was bound to, and nothing until it is
	// bound: it classifies its variants abstractly, so it is no object of itself.
	if ctx.model.IsVariationFeature(slot.Feature.Symbol) {
		if slot.Feature.DefaultValue == nil {
			return nil, fmt.Errorf("%w: %s.%s", ErrVariationUnselected, inst.Type.Name, name)
		}
		val, err := ctx.evalSlotDefault(inst, slot, name)
		if err != nil {
			return nil, err
		}
		bound, err := ctx.bindVariation(slot.Feature, val, inst.ID)
		if err != nil {
			return nil, fmt.Errorf("slot %s.%s: %w", inst.Type.Name, name, err)
		}
		slot.Value = bound
		slot.Materialized = true
		return slot, nil
	}

	// A bound value supplies the feature's own features, so a body restating one
	// of them states two values for it.
	if restated := ctx.restatedInValuedBody(slot.Feature); restated != "" {
		return nil, fmt.Errorf("slot %s.%s: %w: %s", inst.Type.Name, name, ErrValuedFeatureRestated, restated)
	}

	// A default that did not constant-fold is a derived value: evaluate it
	// against this instance, so that it sees the sibling slots it refers to.
	// The feature holds what the default states, once that conforms to the
	// feature's multiplicity.
	if slot.Feature.DefaultValue != nil {
		val, err := ctx.evalSlotDefault(inst, slot, name)
		if err != nil {
			return nil, err
		}
		if err := ctx.checkDefaultCount(inst, slot, name, val); err != nil {
			return nil, err
		}
		if isScalarFeature(slot.Feature) {
			slot.Value = val
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
			slot.Values = val
		}
		slot.Materialized = true
		return slot, nil
	}

	// A connector holds the features it connects at its ends rather than objects
	// of its own, so it is materialized from what the `connect` clause names.
	if ctx.model.IsConnectorUsage(slot.Feature.Symbol) {
		if err := ctx.materializeConnectorSlot(inst, slot, name); err != nil {
			return nil, err
		}
		return slot, nil
	}

	// Lazy instantiation: a composite feature holds objects of its own.
	if composite := ctx.CompositeTypeOf(slot.Feature); composite != nil {
		// Check multiplicity (C2 + C1)
		mult := slot.Feature.Multiplicity
		if !mult.Upper.Known || !mult.Lower.Known {
			return nil, fmt.Errorf("cannot materialize slot %q with unknown multiplicity", name)
		}

		if !mult.Upper.Infinite && mult.Upper.Value == 1 {
			// Scalar: instantiate one
			childInst, err := ctx.Instantiate(composite)
			if err != nil {
				return nil, err
			}
			slot.Value = Value{Kind: ValInstance, Instance: childInst.ID}
		} else {
			// Guard against infinite/huge lower bound (C3)
			if mult.Lower.Infinite || mult.Lower.Value > maxMaterializedLowerBound {
				return nil, fmt.Errorf("%w: lower bound too large or infinite for slot %q", ErrMultiplicityViolation, name)
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
			slot.Values = Value{Kind: ValSequence, Sequence: seq}
		}
		slot.Materialized = true
	}

	return slot, nil
}

// CompositeTypeOf returns what a feature is materialized from, or nil for one
// that holds a value rather than an object — any default takes precedence
// over instantiation, as in GetSlot above. A usage with features of its own is
// instantiated as itself, so its body governs and an untyped nested part
// materializes at all. Answering costs no allocation, so a caller walking an
// object graph can decide whether to descend before descending.
func (ctx *Context) CompositeTypeOf(feat *EffectiveFeature) *symbols.Symbol {
	if feat.DefaultValue != nil {
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

// restatedInValuedBody returns the name of a feature valued again in the body of
// the declaration that binds a value to it, or "" when there is none: the bound
// value supplies the features, so a second value for one could only be dropped.
// A declaration whose body only re-declares features states no second value,
// and neither does a body over a value the redefined declaration wrote.
func (ctx *Context) restatedInValuedBody(feat *EffectiveFeature) string {
	if feat.Symbol == nil {
		return ""
	}
	decl, ok := feat.Symbol.Decl.(*ast.Usage)
	if !ok || decl.Value == nil {
		return ""
	}
	inherited := make(map[string]bool)
	for _, f := range ctx.FeaturesOf(feat.Type) {
		inherited[f.Name] = true
	}
	for _, member := range declMembers(feat.Symbol.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok || (usage.Value == nil && len(declMembers(usage)) == 0) {
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

// evalSlotDefault evaluates a slot's default-value expression bound to the
// owning instance. Recursion back through the slot being computed is reported
// as ErrCyclicSlot rather than recursing until the step budget runs out.
func (ctx *Context) evalSlotDefault(inst *Instance, slot *Slot, name string) (Value, error) {
	key := slotRef{instance: inst.ID, feature: name}
	if ctx.derivingSlots[key] {
		return Value{}, fmt.Errorf("%w: %s.%s", ErrCyclicSlot, inst.Type.Name, name)
	}
	ctx.derivingSlots[key] = true
	defer delete(ctx.derivingSlots, key)

	scope := slot.Feature.DefaultScope()
	if scope == nil {
		scope = inst.Type.OwnerScope
	}
	ec := NewEvalContextIn(ctx, scope, inst)
	defer ec.beginStep()()
	val, err := ec.Eval(slot.Feature.DefaultValue)
	if err != nil {
		return Value{}, fmt.Errorf("slot %s.%s: %w", inst.Type.Name, name, err)
	}
	return val, nil
}
