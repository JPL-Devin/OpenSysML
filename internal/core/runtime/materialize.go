package runtime

import "github.com/Open-MBEE/Systemica/internal/core/symbols"

// maxMaterializeDepth bounds how deep a materialization walk descends into the
// objects an object holds, as a slot listing is bounded.
const maxMaterializeDepth = 8

// MaterializationErrors reads every slot of an object, and of the objects its
// slots hold, and returns what materializing them reported, in the order the
// slots were read. Slots are lazy, so a default that does not conform to its
// feature's multiplicity is only found by reading it: a caller reporting on an
// object it created calls this rather than leaving those diagnostics to whoever
// reads a slot next.
func (ctx *Context) MaterializationErrors(inst *Instance) []error {
	if inst == nil {
		return nil
	}
	w := &materializeWalk{
		ctx:     ctx,
		onPath:  map[*symbols.Symbol]bool{inst.Type: true},
		visited: map[int64]bool{inst.ID: true},
	}
	w.walk(inst, 0)
	return w.errs
}

// materializeWalk reads an object graph under the bounds a slot listing uses:
// onPath holds the types being expanded above the current one, since a part
// containing its own kind materializes a fresh object per descent, and depth
// bounds the descent itself. visited keeps an object two features hold from
// being reported twice.
type materializeWalk struct {
	ctx     *Context
	onPath  map[*symbols.Symbol]bool
	visited map[int64]bool
	errs    []error
}

func (w *materializeWalk) walk(inst *Instance, depth int) {
	features := w.ctx.FeaturesOf(inst.Type)
	for i := range features {
		feat := &features[i]
		// A constraint or requirement a part carries holds a verdict about the
		// object rather than a value, so there is nothing to materialize.
		if holdsVerdict(feat) {
			continue
		}
		if held := w.ctx.CompositeTypeOf(feat); held != nil && (depth >= maxMaterializeDepth || w.onPath[held]) {
			continue
		}
		slot, err := inst.GetSlot(w.ctx, feat.Name)
		if err != nil {
			w.errs = append(w.errs, err)
			continue
		}
		for _, nested := range heldInstances(w.ctx, slot) {
			if w.visited[nested.ID] {
				continue
			}
			w.visited[nested.ID] = true
			w.onPath[nested.Type] = true
			w.walk(nested, depth+1)
			delete(w.onPath, nested.Type)
		}
	}
}

// holdsVerdict reports whether a feature is a condition about the object that
// carries it rather than a feature holding values.
func holdsVerdict(feat *EffectiveFeature) bool {
	if feat.Symbol == nil {
		return false
	}
	switch feat.Symbol.Kind {
	case symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage:
		return true
	default:
		return false
	}
}

// heldInstances returns the objects a slot holds, whether it carries one value
// or a collection of them.
func heldInstances(ctx *Context, slot *Slot) []*Instance {
	held := slot.HeldValue()
	values := []Value{held}
	switch held.Kind {
	case ValSequence:
		values = held.Sequence.Elements()
	case ValSet:
		values = held.Set.Elements()
	}

	var out []*Instance
	for _, val := range values {
		id, ok := val.Object()
		if !ok {
			continue
		}
		if nested, ok := ctx.Instance(id); ok {
			out = append(out, nested)
		}
	}
	return out
}
