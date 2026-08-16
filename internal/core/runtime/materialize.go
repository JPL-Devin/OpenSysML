package runtime

import "github.com/Open-MBEE/Systemica/internal/core/symbols"

const (
	// maxMaterializeDepth bounds how deep a materialization walk descends into the
	// objects an object holds, as a slot listing is bounded.
	maxMaterializeDepth = 8
	// maxMaterializeBudget bounds the walk as a whole, charged per slot read and
	// per object a slot holds: nesting multiplies, and reading a slot materializes
	// the objects it holds, so breadth costs objects rather than only time.
	maxMaterializeBudget = 1000
)

// MaterializationErrors reads every slot of an object, and of the objects its
// slots hold, and returns what materializing them reported, in the order the
// slots were read. Slots are lazy, so a default that does not conform to its
// feature's multiplicity is only found by reading it: a caller reporting on an
// object it created calls this rather than leaving those diagnostics to whoever
// reads a slot next. bounded is true when the walk spent its budget before
// every slot was read, so what it did not reach is unreported rather than clean.
func (ctx *Context) MaterializationErrors(inst *Instance) (errs []error, bounded bool) {
	if inst == nil {
		return nil, false
	}
	w := &materializeWalk{
		ctx:     ctx,
		onPath:  map[*symbols.Symbol]bool{inst.Type: true},
		visited: map[int64]bool{inst.ID: true},
		read:    map[*Slot]bool{},
		budget:  maxMaterializeBudget,
	}
	w.walk(inst, 0)
	return w.errs, w.bounded
}

// materializeWalk reads an object graph under the bounds a slot listing uses:
// onPath holds the types being expanded above the current one, since a part
// containing its own kind materializes a fresh object per descent; depth bounds
// the descent and budget the walk as a whole. visited keeps an object two
// features hold from being reported twice, and read the one slot a feature and
// the feature redefining it share.
type materializeWalk struct {
	ctx     *Context
	onPath  map[*symbols.Symbol]bool
	visited map[int64]bool
	read    map[*Slot]bool
	budget  int
	bounded bool
	errs    []error
}

func (w *materializeWalk) walk(inst *Instance, depth int) {
	features := w.ctx.FeaturesOf(inst.Type)
	for i := range features {
		if w.budget <= 0 {
			w.bounded = true
			return
		}
		feat := &features[i]
		// A constraint or requirement a part carries holds a verdict about the
		// object rather than a value, so there is nothing to materialize.
		if holdsVerdict(feat) {
			continue
		}
		if held := w.ctx.CompositeTypeOf(feat); held != nil && (depth >= maxMaterializeDepth || w.onPath[held]) {
			continue
		}
		// A redefinition names the redefined feature again, and the two names read
		// one slot, so reading it once reports what it holds once.
		if shared := inst.Slots[feat.Name]; shared != nil {
			if w.read[shared] {
				continue
			}
			w.read[shared] = true
		}
		w.budget--
		slot, err := inst.GetSlot(w.ctx, feat.Name)
		if err != nil {
			w.errs = append(w.errs, err)
			continue
		}
		nested := heldInstances(w.ctx, slot)
		w.budget -= len(nested)
		for _, held := range nested {
			if w.budget <= 0 {
				w.bounded = true
				return
			}
			if w.visited[held.ID] {
				continue
			}
			w.visited[held.ID] = true
			w.onPath[held.Type] = true
			w.walk(held, depth+1)
			delete(w.onPath, held.Type)
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
