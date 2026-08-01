package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
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

// EvaluateConstraint evaluates a constraint definition/usage.
// Returns (satisfied, error). If IsAssert=true, violation is an error.
// If IsAssert=false (assume), always returns (true, nil) but logs assumptions.
func (ctx *Context) EvaluateConstraint(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	// Extract constraint members
	var members []ast.Node
	
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind != ast.DefConstraint {
			return false, fmt.Errorf("not a constraint definition: %s", sym.Name)
		}
		members = decl.Members
	case *ast.Usage:
		if decl.Kind != ast.UsageConstraint {
			return false, fmt.Errorf("not a constraint usage: %s", sym.Name)
		}
		members = decl.Members
	default:
		return false, fmt.Errorf("invalid constraint symbol: %s (%T)", sym.Name, sym.Decl)
	}
	
	// Evaluate each constraint member
	for _, member := range members {
		// Unwrap Membership
		node := member
		if membership, ok := member.(*ast.Membership); ok {
			node = membership.Member
		}
		
		// Check for ConstraintMember
		constraintMember, ok := node.(*ast.ConstraintMember)
		if !ok {
			continue // skip non-constraint members
		}
		
		// Evaluate constraint expression
		result, err := ctx.EvalWithScope(constraintMember.Expression, scope)
		if err != nil {
			return false, fmt.Errorf("constraint %s: evaluation failed: %w", sym.Name, err)
		}
		
		// Extract boolean value
		satisfied := false
		if result.Kind == ValConst && result.Const.Kind == semantics.ValBool {
			satisfied = result.Const.Bool
		} else {
			return false, fmt.Errorf("constraint %s: expression must evaluate to boolean, got %v", sym.Name, result.Kind)
		}
		
		// Apply negation
		if constraintMember.IsNegated {
			satisfied = !satisfied
		}
		
		// Handle assert vs assume
		if constraintMember.IsAssert {
			if !satisfied {
				return false, fmt.Errorf("constraint %s: assertion failed", sym.Name)
			}
		}
		// assume: always pass (assumptions are trusted)
	}
	
	return true, nil
}
