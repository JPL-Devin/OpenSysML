package runtime

// A `=` value lists itself as a dependent of each feature value it reads, and a
// change to one unmaterializes its dependents, transitively, to derive again when read.

// derivable reports whether fv's value is derived from what it reads: bound by `=`,
// neither a fallback default nor assigned by a run nor propagated by a binding.
func (ctx *Context) derivable(fv *FeatureValue) bool {
	return !fv.Written && !fv.BindingDerived && !fv.Feature.DefaultIsFallback()
}

// deriveFeatureValue evaluates fv's `=` expression, recording what it reads.
func (ctx *Context) deriveFeatureValue(inst *Instance, fv *FeatureValue, name string) (Value, error) {
	if !ctx.derivable(fv) {
		return ctx.evalFeatureValueDefault(inst, fv, name)
	}
	ctx.deriving = append(ctx.deriving, fv)
	defer func() { ctx.deriving = ctx.deriving[:len(ctx.deriving)-1] }()
	return ctx.evalFeatureValueDefault(inst, fv, name)
}

// noteRead lists the value being derived, if any, as a dependent of the fv just read.
func (ctx *Context) noteRead(fv *FeatureValue) {
	if len(ctx.deriving) == 0 {
		return
	}
	dep := ctx.deriving[len(ctx.deriving)-1]
	if dep == fv {
		return
	}
	for _, listed := range fv.dependents {
		if listed == dep {
			return
		}
	}
	ctx.noteProbeWrite(fv)
	fv.dependents = append(fv.dependents, dep)
}

// priorValue is what fv holds before a write, and whether anything depends on it.
func (ctx *Context) priorValue(fv *FeatureValue) (Value, bool) {
	if len(fv.dependents) == 0 {
		return Value{}, false
	}
	return fv.HeldValue(), true
}

// noteChanged unmaterializes what depended on fv before a write, unless fv still
// holds prior; a dependent recorded during the write read what fv holds now.
func (ctx *Context) noteChanged(fv *FeatureValue, prior Value, had bool) {
	if !had || len(fv.dependents) == 0 || (fv.Materialized && heldSame(prior, fv.HeldValue())) {
		return
	}
	ctx.invalidateDependents(fv)
}

// heldSame reports whether now is prior as a derivation reads it: `===`, and for a
// quantity the same unit too, which an operation over it carries into its result.
func heldSame(prior, now Value) bool {
	if isEmptyValue(prior) || isEmptyValue(now) {
		return isEmptyValue(prior) && isEmptyValue(now)
	}
	if prior.Kind != now.Kind {
		return false
	}
	switch prior.Kind {
	case ValQuantity:
		return prior.Quantity().Unit.Product.Equal(now.Quantity().Unit.Product) && valueIdentical(prior, now)
	case ValVectorQuantity:
		return vectorQuantityHeldSame(prior.VectorQuantity(), now.VectorQuantity())
	case ValSequence:
		return elementsHeldSame(prior.Sequence().Elements(), now.Sequence().Elements())
	case ValArray:
		return valueIdentical(prior, now) && elementsHeldSame(prior.Array().Elements, now.Array().Elements)
	case ValSet:
		return setHeldSame(prior.Set(), now.Set())
	}
	return valueIdentical(prior, now)
}

func vectorQuantityHeldSame(prior, now *VectorQuantity) bool {
	if prior == nil || now == nil || prior.Dimension() != now.Dimension() {
		return prior == now
	}
	for i := range prior.Num {
		if !heldSame(NewQuantityValue(prior.component(i)), NewQuantityValue(now.component(i))) {
			return false
		}
	}
	return true
}

func elementsHeldSame(prior, now []Value) bool {
	if len(prior) != len(now) {
		return false
	}
	for i := range prior {
		if !heldSame(prior[i], now[i]) {
			return false
		}
	}
	return true
}

func setHeldSame(prior, now *Set) bool {
	if prior == nil || now == nil || prior.Size() != now.Size() {
		return prior == now
	}
	rights := now.Elements()
	used := make([]bool, len(rights))
	for _, left := range prior.Elements() {
		found := false
		for i, right := range rights {
			if !used[i] && heldSame(left, right) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// invalidateDependents unmaterializes what was derived from fv, transitively. Each
// list is detached before it is walked, so a cycle ends the walk; a value being
// derived right now stays listed, as fv changed under it.
func (ctx *Context) invalidateDependents(fv *FeatureValue) {
	if len(fv.dependents) == 0 {
		return
	}
	ctx.noteProbeWrite(fv)
	dependents := fv.dependents
	fv.dependents = nil
	var deriving []*FeatureValue
	for _, dep := range dependents {
		switch {
		case ctx.isDeriving(dep):
			deriving = append(deriving, dep)
		case dep.Written || dep.BindingDerived:
			// Assigned or bound since: its value is no longer derived from fv.
		case !dep.Materialized:
			ctx.invalidateDependents(dep)
		default:
			ctx.noteProbeWrite(dep)
			dep.Value, dep.Values, dep.Materialized = Value{}, Value{}, false
			ctx.invalidateDependents(dep)
		}
	}
	fv.dependents = deriving
}

// isDeriving reports whether fv is being derived right now.
func (ctx *Context) isDeriving(fv *FeatureValue) bool {
	for _, deriving := range ctx.deriving {
		if deriving == fv {
			return true
		}
	}
	return false
}
