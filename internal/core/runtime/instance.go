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

		// Evaluate default value if present and scalar
		if feat.DefaultValue != nil && feat.Multiplicity.Upper.Value <= 1 {
			// Use semantics.Eval for constant defaults (Tier 3 will use full evaluator)
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
