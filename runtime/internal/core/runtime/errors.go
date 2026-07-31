package runtime

import (
	"errors"
	"fmt"
)

var (
	// ErrStepLimitExceeded is returned when the evaluation step counter exceeds maxSteps.
	ErrStepLimitExceeded = errors.New("evaluation step limit exceeded")

	// ErrUnresolvedReference is returned when a feature reference cannot be resolved.
	ErrUnresolvedReference = errors.New("unresolved reference")

	// ErrTypeMismatch is returned when an operation receives a value of unexpected type.
	ErrTypeMismatch = errors.New("type mismatch")

	// ErrMultiplicityViolation is returned when a slot access/assignment violates multiplicity bounds.
	ErrMultiplicityViolation = errors.New("multiplicity violation")

	// ErrUninitializedSlot is returned when accessing a slot that has no value and no default.
	ErrUninitializedSlot = errors.New("uninitialized slot")
)

// EvalError wraps an evaluation error with source context.
type EvalError struct {
	Msg string
	Err error
}

func (e *EvalError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *EvalError) Unwrap() error {
	return e.Err
}
