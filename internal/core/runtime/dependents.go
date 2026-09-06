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
	if !had || len(fv.dependents) == 0 || (fv.Materialized && valueIdentical(prior, fv.HeldValue())) {
		return
	}
	ctx.invalidateDependents(fv)
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
