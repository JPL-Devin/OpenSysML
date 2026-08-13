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

	// ErrUnresolvedFeature is returned when an unqualified name in an expression
	// names nothing the expression's scope, its frames or its object supply, so a
	// value the model never declared is reported rather than assumed.
	ErrUnresolvedFeature = errors.New("unresolved feature")

	// ErrTypeMismatch is returned when an operation receives a value of unexpected type.
	ErrTypeMismatch = errors.New("type mismatch")

	// ErrMultiplicityViolation is returned when a slot access/assignment violates multiplicity bounds.
	ErrMultiplicityViolation = errors.New("multiplicity violation")

	// ErrUninitializedSlot is returned when accessing a slot that has no value and no default.
	ErrUninitializedSlot = errors.New("uninitialized slot")

	// ErrNotACalc is returned when a calc invocation targets a symbol that is
	// not a calc definition or usage.
	ErrNotACalc = errors.New("not a calc")

	// ErrCalcArity is returned when a calc invocation passes more arguments than
	// the calc declares input parameters.
	ErrCalcArity = errors.New("calc argument count mismatch")

	// ErrUnboundParameter is returned when a calc input parameter receives
	// neither an argument nor a declared default.
	ErrUnboundParameter = errors.New("unbound parameter")

	// ErrUnknownParameter is returned when a named argument does not name any
	// input parameter of the invoked calc.
	ErrUnknownParameter = errors.New("unknown parameter")

	// ErrNoResultExpression is returned when a calc body declares no return
	// expression, directly or by inheritance.
	ErrNoResultExpression = errors.New("no result expression")

	// ErrUnsupportedOperator is returned when an operator has no runtime
	// evaluation, so an expression naming it fails rather than yielding nothing.
	ErrUnsupportedOperator = errors.New("unsupported operator")

	// ErrCalcNoReturn is returned when a calc body runs to its end without
	// returning: it computed no result, which is not the same as a null one.
	ErrCalcNoReturn = errors.New("calculation returned no value")

	// ErrCalcSideEffect is returned when a calc body states an effect on the
	// world outside it — send, perform, accept, terminate. A calculation
	// computes a value, so an effect is rejected rather than performed.
	ErrCalcSideEffect = errors.New("side effect in a calculation body")

	// ErrCalcExternalAssignment is returned when a calc body assigns to a name it
	// does not declare itself, which would make the calculation impure.
	ErrCalcExternalAssignment = errors.New("assignment outside the calculation body")

	// ErrReturnOutsideCalc is returned when a `return` is executed by a host that
	// has no result to return, an action node's body.
	ErrReturnOutsideCalc = errors.New("'return' outside a calculation body")

	// ErrAcceptDeadlock is returned when an action can no longer progress
	// because every token it has left is parked at an accept, so no token can
	// post the message any of them waits for. An accept suspends the action
	// rather than failing, so this is how a suspension that can never end is
	// reported instead of hanging.
	ErrAcceptDeadlock = errors.New("accept deadlock")

	// ErrCalcRecursionLimit is returned when calc invocation nests deeper than
	// maxCalcNestingDepth, which a recursive calc would otherwise do until the
	// process ran out of stack.
	ErrCalcRecursionLimit = errors.New("calc recursion limit exceeded")

	// ErrViolated is returned when an asserted constraint or a required
	// condition evaluates to false. It is a verdict about the model, not a
	// failure to evaluate, so callers can tell the two apart.
	ErrViolated = errors.New("evaluated to false")

	// ErrNoValue is returned when a feature a condition names carries no value:
	// neither a slot on the object being checked nor a declared default.
	ErrNoValue = errors.New("no value")

	// ErrNoConditions is returned when a constraint or requirement carries no
	// condition to evaluate: reporting a verdict would claim a check that never ran.
	ErrNoConditions = errors.New("no condition to evaluate")

	// ErrCyclicSlot is returned when a slot's default value depends, directly or
	// through other slots, on the slot being computed.
	ErrCyclicSlot = errors.New("cyclic slot dependency")

	// ErrNotAQuantity is returned when `x [y]` is not a quantity expression:
	// y names no measurement unit, or x is no magnitude.
	ErrNotAQuantity = errors.New("not a quantity expression")

	// ErrIncommensurableUnits is returned when an operation combines quantities
	// whose units measure different things, or whose conversion is not derivable
	// from the library. It is never answered by comparing magnitudes.
	ErrIncommensurableUnits = errors.New("incommensurable units")

	// ErrNotASatisfaction is returned when a satisfaction assertion is asked of
	// an element that states none.
	ErrNotASatisfaction = errors.New("not a satisfaction assertion")

	// ErrNoRequirement is returned when a satisfaction assertion states no
	// requirement to evaluate: it references none, or references one that
	// resolves to nothing.
	ErrNoRequirement = errors.New("no requirement to satisfy")

	// ErrNoSubject is returned when the feature a satisfaction assertion names
	// with `by` cannot supply a subject: it resolves to nothing, or no object of
	// it can be created.
	ErrNoSubject = errors.New("no subject to satisfy the requirement")
)

// ViolationError reports a condition that evaluated to false, naming the
// condition so a verdict says which one failed. It unwraps to ErrViolated,
// since it is a verdict about the model rather than a failure to evaluate.
type ViolationError struct {
	Kind      string // "constraint" or "requirement"
	Element   string // name of the element stating the condition
	What      string // "assertion" or "require condition"
	Condition string // the condition, rendered
}

func (e *ViolationError) Error() string {
	return fmt.Sprintf("%s %s: %s %v: %s", e.Kind, e.Element, e.What, ErrViolated, e.Condition)
}

func (e *ViolationError) Unwrap() error { return ErrViolated }

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
