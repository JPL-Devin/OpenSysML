package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Instance is a runtime-materialized object (Tier 2).
type Instance struct {
	ID    int64            // unique identity
	Type  *symbols.Symbol  // the def/usage symbol this instantiates
	Slots map[string]*Slot // feature name → slot
}

// Slot holds the runtime value(s) for one feature.
type Slot struct {
	Feature      *EffectiveFeature
	Value        Value // scalar slot (multiplicity [1])
	Values       Value // collection slot (Sequence or Set)
	Materialized bool  // lazy flag: has this slot been instantiated?
}

// Instantiate materializes an instance of the given usage/definition symbol.
// Allocates ID, creates slots per FeaturesOf(sym), evaluates default values,
// leaves composite features lazy. Returns the instance or an error.
func (ctx *Context) Instantiate(sym *symbols.Symbol) (*Instance, error) {
	defer ctx.beginRun()()

	// Check step limit (I3)
	if err := ctx.incrementStep(); err != nil {
		return nil, err
	}

	// Allocate ID
	id := ctx.allocateID()

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
		if feat.DefaultValue != nil && isScalarFeature(feat) && !ctx.model.IsVariationFeature(feat.Symbol) {
			if semVal, ok := ctx.model.Eval(feat.DefaultValue); ok {
				slot.Value = Value{Kind: ValConst, Const: semVal}
				slot.Materialized = true
			}
		}

		inst.Slots[feat.Name] = slot
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
	mult := ctx.extractMultiplicity(sym)
	return !mult.Upper.Infinite && mult.Upper.Value <= 1
}

// isScalarFeature reports whether a feature holds at most one value. An
// unbounded upper bound carries Value 0, so the infinite flag has to be tested
// separately.
func isScalarFeature(feat *EffectiveFeature) bool {
	return !feat.Multiplicity.Upper.Infinite && feat.Multiplicity.Upper.Value <= 1
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
		bound, err := ctx.bindVariation(slot.Feature, val)
		if err != nil {
			return nil, fmt.Errorf("slot %s.%s: %w", inst.Type.Name, name, err)
		}
		slot.Value = bound
		slot.Materialized = true
		return slot, nil
	}

	// A default that did not constant-fold is a derived value: evaluate it
	// against this instance, so that it sees the sibling slots it refers to.
	if slot.Feature.DefaultValue != nil && isScalarFeature(slot.Feature) {
		val, err := ctx.evalSlotDefault(inst, slot, name)
		if err != nil {
			return nil, err
		}
		slot.Value = val
		slot.Materialized = true
		return slot, nil
	}

	// A multi-valued feature given a default holds that default's contents; a
	// single value written there is the collection's one element.
	if slot.Feature.DefaultValue != nil && slot.Feature.Type == nil {
		val, err := ctx.evalSlotDefault(inst, slot, name)
		if err != nil {
			return nil, err
		}
		if val.Kind != ValSequence && val.Kind != ValSet {
			if err := ctx.chargeElements(1); err != nil {
				return nil, err
			}
			seq := NewSequence()
			seq.Append(val)
			val = Value{Kind: ValSequence, Sequence: seq}
		}
		slot.Values = val
		slot.Materialized = true
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
			// Collection: instantiate up to lower bound (or 0 if unbounded)
			count := int(mult.Lower.Value)

			// Guard against infinite/huge lower bound (C3)
			if mult.Lower.Infinite || mult.Lower.Value > 1000 {
				return nil, fmt.Errorf("lower bound too large or infinite for slot %q", name)
			}

			if count < 0 {
				count = 0
			}

			// Determine collection type (Sequence vs Set)
			if err := ctx.chargeElements(int64(count)); err != nil {
				return nil, err
			}
			seq := NewSequence()
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
// that holds a value rather than an object — a written default takes precedence
// over instantiation, as in GetSlot above. A usage with members of its own is
// instantiated as itself, so its body governs and an untyped nested part
// materializes at all. Answering costs no allocation, so a caller walking an
// object graph can decide whether to descend before descending.
func (ctx *Context) CompositeTypeOf(feat *EffectiveFeature) *symbols.Symbol {
	if feat.DefaultValue != nil && (isScalarFeature(feat) || feat.Type == nil) {
		return nil
	}
	// A variation is materialized from the variant it is bound to, never from
	// itself: it is an abstract classifier of its variants.
	if ctx.model.IsVariationFeature(feat.Symbol) {
		return nil
	}
	if feat.Symbol != nil && isCompositeUsage(feat.Symbol) && len(declMembers(feat.Symbol.Decl)) > 0 {
		return feat.Symbol
	}
	return feat.Type
}

// isCompositeUsage reports whether a feature symbol holds objects (a part or
// item) rather than a value.
func isCompositeUsage(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolPartUsage, symbols.SymbolItemUsage:
		return true
	default:
		return false
	}
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

	scope := slot.Feature.DeclScope()
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
