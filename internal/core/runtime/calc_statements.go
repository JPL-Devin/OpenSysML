package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/lower"
)

// calcStmtHost runs a calculation body's statements: it owns its locals and its
// returned value, and rejects every statement acting on the world outside it.
type calcStmtHost struct {
	result Value // the value its `return` yielded
}

// describe names the body in a diagnostic; the invocation adds the calc's name.
func (h *calcStmtHost) describe() string {
	return "calculation body"
}

func (h *calcStmtHost) send(*EvalContext, lower.Send) error {
	return fmt.Errorf("%w: a calculation cannot send a message", ErrCalcSideEffect)
}

// assignOuter rejects an assignment to a name the calculation's parameters and
// body do not declare: writing it would be an effect outside the calculation.
func (h *calcStmtHost) assignOuter(_ *stmtEnv, name string, _ Value, _ lower.Assign) error {
	return fmt.Errorf("%w: %s is not declared by the calculation", ErrCalcExternalAssignment, name)
}

func (h *calcStmtHost) acceptReturn(value Value, _ lower.Return) error {
	h.result = value
	return nil
}

func (h *calcStmtHost) effect(s lower.Effect) error {
	return fmt.Errorf("%w: a calculation cannot state '%s'", ErrCalcSideEffect, s.Kind)
}
