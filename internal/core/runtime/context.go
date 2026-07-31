package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Instance is a placeholder for Tier 2. Stub allows Context to compile.
type Instance struct {
	ID int64
}

// EffectiveFeature is a placeholder for Tier 1. Stub allows Context to compile.
type EffectiveFeature struct{}


// Context carries runtime execution state. One per workspace session.
type Context struct {
	model     *semantics.Model
	nextID    int64
	steps     int64
	maxSteps  int64
	instances map[int64]*Instance
	features  map[*symbols.Symbol][]EffectiveFeature
}

// NewContext creates a runtime context backed by the given semantic model.
// maxSteps sets the runaway guard (step counter limit).
func NewContext(model *semantics.Model, maxSteps int64) *Context {
	return &Context{
		model:     model,
		nextID:    1, // IDs start at 1 (0 = invalid)
		steps:     0,
		maxSteps:  maxSteps,
		instances: make(map[int64]*Instance),
		features:  make(map[*symbols.Symbol][]EffectiveFeature),
	}
}

// allocateID returns the next instance ID and increments the counter.
func (ctx *Context) allocateID() int64 {
	id := ctx.nextID
	ctx.nextID++
	return id
}

// incrementStep increments the step counter and returns ErrStepLimitExceeded if limit reached.
func (ctx *Context) incrementStep() error {
	ctx.steps++
	if ctx.steps > ctx.maxSteps {
		return fmt.Errorf("%w (%d steps)", ErrStepLimitExceeded, ctx.maxSteps)
	}
	return nil
}

// getInstance retrieves an instance by ID.
func (ctx *Context) getInstance(id int64) (*Instance, bool) {
	inst, ok := ctx.instances[id]
	return inst, ok
}

// registerInstance stores an instance in the registry.
func (ctx *Context) registerInstance(inst *Instance) {
	ctx.instances[inst.ID] = inst
}
