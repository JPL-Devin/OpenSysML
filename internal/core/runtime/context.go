package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Context carries runtime execution state. One per workspace session.
type Context struct {
	model     *semantics.Model
	resolver  *resolve.Resolver
	nextID    int64
	steps     int64
	maxSteps  int64
	instances map[int64]*Instance
	features  map[*symbols.Symbol][]EffectiveFeature
}

// NewContext creates a runtime context backed by the given semantic model.
// maxSteps sets the runaway guard (step counter limit).
func NewContext(model *semantics.Model, resolver *resolve.Resolver, maxSteps int64) *Context {
	if maxSteps <= 0 {
		panic(fmt.Sprintf("runtime: maxSteps must be > 0, got %d", maxSteps))
	}
	return &Context{
		model:     model,
		resolver:  resolver,
		nextID:    1, // IDs start at 1 (0 = invalid)
		steps:     0,
		maxSteps:  maxSteps,
		instances: make(map[int64]*Instance),
		features:  make(map[*symbols.Symbol][]EffectiveFeature),
	}
}

// Model returns the semantic model this context operates over.
func (ctx *Context) Model() *semantics.Model {
	return ctx.model
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
	if inst.ID <= 0 {
		panic(fmt.Sprintf("runtime: invalid instance ID %d (must be > 0)", inst.ID))
	}
	if _, exists := ctx.instances[inst.ID]; exists {
		panic(fmt.Sprintf("runtime: duplicate instance ID %d", inst.ID))
	}
	ctx.instances[inst.ID] = inst
}
