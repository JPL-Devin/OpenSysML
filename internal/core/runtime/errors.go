package runtime

import (
	"errors"
	"fmt"
)

var (
	// ErrStepLimitExceeded is returned when the evaluation step counter exceeds maxSteps.
	ErrStepLimitExceeded = errors.New("evaluation step limit exceeded")

	// ErrElementLimitExceeded is returned when the collection elements one run
	// materializes exceed maxElements. It is a bound on memory rather than on
	// work, so it is its own error and its own budget.
	ErrElementLimitExceeded = errors.New("collection element limit exceeded")

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

	// ErrNotACalcUsage is returned when an output feature is read from a symbol
	// that is not a calc usage: only a usage carries an evaluation whose outputs
	// are features.
	ErrNotACalcUsage = errors.New("not a calc usage")

	// ErrUnknownOutput is returned when a name read from a calc usage is not one
	// of the output features its calc declares.
	ErrUnknownOutput = errors.New("unknown output")

	// ErrCyclicOutput is returned when an output feature's binding depends,
	// directly or through other outputs, on the output being computed.
	ErrCyclicOutput = errors.New("cyclic output dependency")

	// ErrAmbiguousResult is returned when a calc declaring several output
	// features is invoked as an expression. A function invocation has exactly
	// one result (KerML 7.4.9), so a calc that designates none has no value to
	// hand back and is read through a calc usage's output features instead.
	ErrAmbiguousResult = errors.New("calculation has no single result")

	// ErrIndexOutOfRange is returned when a sequence index names no position of
	// the sequence it indexes. Sequence indices are 1-based (KerML
	// SequenceFunctions::'#' takes `index: Positive[1]`), so 0 is out of range
	// as much as size+1 is.
	ErrIndexOutOfRange = errors.New("sequence index out of range")

	// ErrBodyArity is returned when the body expression a collection operation
	// is given declares a number of parameters the operation cannot call it
	// with: `select` calls its selector with one element, so a selector
	// declaring two parameters has no second argument to receive.
	ErrBodyArity = errors.New("body parameter count mismatch")

	// ErrReceiverWithNamedArgs is returned when a receiver is written before a
	// call whose arguments are named, `x->f(a = 1)`. The receiver binds by
	// position and the arguments by name, so which parameter the receiver binds
	// to is unstated; it is reported rather than dropped.
	ErrReceiverWithNamedArgs = errors.New("receiver combined with named arguments")

	// ErrVariationUnselected is returned when a variation is read without having
	// been bound to one of its variants: it classifies its variants abstractly,
	// so it stands for no one value until a variant is selected.
	ErrVariationUnselected = errors.New("variation has no variant selected")

	// ErrNotAVariant is returned when a variation is bound to something that is
	// not one of the variants it offers.
	ErrNotAVariant = errors.New("not a variant of the variation")

	// ErrMultipleVariants is returned when a variation is bound to more than one
	// variant, which selects no single configuration.
	ErrMultipleVariants = errors.New("more than one variant selected")

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
