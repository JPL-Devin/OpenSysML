package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/lower"
)

// calcStmtHost runs a calculation body's statements: it owns its locals, its
// output features and its returned value, and rejects every outside effect.
type calcStmtHost struct {
	shape  *calcShape
	result Value // the value its `return` yielded
	// bound are the outputs this activation assigned, so a second assignment to
	// one is reported rather than overwriting it.
	bound map[string]bool
}

// describe names the body in a diagnostic; the invocation adds the calc's name.
func (h *calcStmtHost) describe() string {
	return "calculation body"
}

func (h *calcStmtHost) send(*EvalContext, lower.Send) error {
	return fmt.Errorf("%w: a calculation cannot send a message", ErrCalcSideEffect)
}

// declaredOutput reports whether name is an output the body binds by assigning
// it. An `inout` is bound by the invocation, so writing it writes a parameter.
func (h *calcStmtHost) declaredOutput(name string) bool {
	if h.shape == nil {
		return false
	}
	out, ok := h.shape.output(name)
	return ok && !out.IsInOut
}

// assignOuter binds an output this calculation declares, and rejects any other
// undeclared name: writing that would be an effect outside the calculation.
func (h *calcStmtHost) assignOuter(env *stmtEnv, name string, value Value, _ lower.Assign) error {
	if !h.declaredOutput(name) {
		return fmt.Errorf("%w: %s is not declared by the calculation", ErrCalcExternalAssignment, name)
	}
	out, _ := h.shape.output(name)
	if out.Value != nil {
		return fmt.Errorf(
			"%w: output %s of calc %s is both given a value by its declaration and assigned in its body",
			ErrConflictingOutput, name, h.shape.Name,
		)
	}
	if h.bound[name] {
		return fmt.Errorf(
			"%w: output %s of calc %s is assigned more than once by one activation",
			ErrConflictingOutput, name, h.shape.Name,
		)
	}
	if h.bound == nil {
		h.bound = make(map[string]bool)
	}
	h.bound[name] = true
	// Written to the body's own data, so later statements read the output bound
	// and the read that follows the activation answers from it.
	env.data[name] = value
	return nil
}

func (h *calcStmtHost) acceptReturn(value Value, _ lower.Return) error {
	h.result = value
	return nil
}

func (h *calcStmtHost) effect(s lower.Effect) error {
	return fmt.Errorf("%w: a calculation cannot state '%s'", ErrCalcSideEffect, s.Kind)
}
