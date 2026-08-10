package runtime

import (
	"fmt"

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
		if feat.DefaultValue != nil && isScalarFeature(feat) {
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

// isScalarFeature reports whether a feature holds at most one value. An
// unbounded upper bound carries Value 0, so the infinite flag has to be tested
// separately.
func isScalarFeature(feat *EffectiveFeature) bool {
	return !feat.Multiplicity.Upper.Infinite && feat.Multiplicity.Upper.Value <= 1
}

// GetSlot retrieves the slot for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet.
func (inst *Instance) GetSlot(ctx *Context, name string) (*Slot, error) {
	slot, ok := inst.Slots[name]
	if !ok {
		return nil, fmt.Errorf("slot %q not found in instance %d (type %s)", name, inst.ID, inst.Type.Name)
	}

	// If already materialized, return
	if slot.Materialized {
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
			seq := NewSequence()
			seq.Append(val)
			val = Value{Kind: ValSequence, Sequence: seq}
		}
		slot.Values = val
		slot.Materialized = true
		return slot, nil
	}

	// Lazy instantiation: if feature is composite (has a type that's a part/item def)
	if slot.Feature.Type != nil {
		// Check multiplicity (C2 + C1)
		mult := slot.Feature.Multiplicity
		if !mult.Upper.Known || !mult.Lower.Known {
			return nil, fmt.Errorf("cannot materialize slot %q with unknown multiplicity", name)
		}

		if !mult.Upper.Infinite && mult.Upper.Value == 1 {
			// Scalar: instantiate one
			childInst, err := ctx.Instantiate(slot.Feature.Type)
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
			seq := NewSequence()
			for i := 0; i < count; i++ {
				childInst, err := ctx.Instantiate(slot.Feature.Type)
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
	val, err := NewEvalContextIn(ctx, scope, inst).Eval(slot.Feature.DefaultValue)
	if err != nil {
		return Value{}, fmt.Errorf("slot %s.%s: %w", inst.Type.Name, name, err)
	}
	return val, nil
}
