package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// calcStmtHost runs a calculation body's statements: it owns its locals, its
// output features and its returned value, and rejects every outside effect.
type calcStmtHost struct {
	ctx    *Context
	shape  *calcShape
	self   *Instance
	result Value // the value its `return` yielded
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
func (h *calcStmtHost) assignOuter(env *stmtEnv, name string, value Value, s lower.Assign) error {
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
	// Written to the body's own data, so later statements read the output bound —
	// an assignment may accumulate into it — and the read that follows the
	// activation answers from what the body left.
	return storeBodyValue(h.ctx, h, env, name, value, s)
}

func (h *calcStmtHost) assignData(env *stmtEnv, name string, value Value, s lower.Assign) error {
	return storeBodyValue(h.ctx, h, env, name, value, s)
}

// assignChain rejects a chained target: writing a feature of another object is
// an effect outside the calculation, as writing an undeclared name is.
func (h *calcStmtHost) assignChain(_ *EvalContext, s lower.Assign, _ Value) error {
	return fmt.Errorf("%w: %s writes a feature of another object", ErrCalcExternalAssignment, s.Chain.Text)
}

// acceptReturn takes the value a `return` yields, which the result parameter
// then holds, so it answers to that parameter's declaration.
func (h *calcStmtHost) acceptReturn(value Value, _ lower.Return) error {
	if out := h.shape.resultOutput(); out != nil {
		if err := out.Decl.checkType(h.ctx, &value, func() string { return "result" }); err != nil {
			return err
		}
	}
	h.result = value
	return nil
}

func (h *calcStmtHost) performer() *Instance {
	// A calculation's steps see the object in its evaluation context, or nil without one.
	return h.self
}

func (h *calcStmtHost) effect(s lower.Effect) error {
	return fmt.Errorf("%w: a calculation cannot state '%s'", ErrCalcSideEffect, s.Kind)
}
