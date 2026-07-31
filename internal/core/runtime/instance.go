package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Instance is a runtime-materialized object (Tier 2).
type Instance struct {
	ID    int64              // unique identity
	Type  *symbols.Symbol    // the def/usage symbol this instantiates
	Slots map[string]*Slot   // feature name → slot
}

// Slot holds the runtime value(s) for one feature.
type Slot struct {
	Feature      *EffectiveFeature
	Value        Value   // scalar slot (multiplicity [1])
	Values       Value   // collection slot (Sequence or Set)
	Materialized bool    // lazy flag: has this slot been instantiated?
}

// GetSlot retrieves the slot for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet.
func (inst *Instance) GetSlot(ctx *Context, name string) (*Slot, error) {
	slot, ok := inst.Slots[name]
	if !ok {
		return nil, fmt.Errorf("slot %q not found in instance %d (type %s)", name, inst.ID, inst.Type.Name)
	}
	
	// Lazy materialization deferred to Task 6
	return slot, nil
}
