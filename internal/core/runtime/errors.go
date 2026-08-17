package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/source"
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

	// ErrTypeMismatch is returned when an operation receives a value of unexpected type.
	ErrTypeMismatch = errors.New("type mismatch")

	// ErrDivisionByZero is returned when a division or remainder has a zero
	// divisor. It is the answer to the expression, not a missing declaration.
	ErrDivisionByZero = errors.New("division by zero")

	// ErrMultiplicityViolation is returned when a slot access/assignment violates multiplicity bounds.
	ErrMultiplicityViolation = errors.New("multiplicity violation")

	// ErrUninitializedSlot is returned when accessing a slot that has no value and no default.
	ErrUninitializedSlot = errors.New("uninitialized feature value")

	// ErrNotACalc is returned when a calc invocation targets a symbol that is
	// not a calc definition or usage.
	ErrNotACalc = errors.New("not a calc")

	// ErrNotAConstraint is returned when a symbol asked to be evaluated as a
	// constraint declares something else. It is a usage error about the request,
	// not a verdict about the model, so callers can tell the two apart.
	ErrNotAConstraint = errors.New("not a constraint")

	// ErrNotARequirement is returned when a symbol asked to be evaluated as a
	// requirement declares something else. Like ErrNotAConstraint it reports the
	// request, not the model.
	ErrNotARequirement = errors.New("not a requirement")

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

	// ErrNoClock is returned when a behavior waits for a time event where no
	// clock advances: an action body has no time base of its own, so
	// `accept at t` / `accept after d` written among an action's nodes is
	// reported rather than passed through as if the instant had arrived.
	ErrNoClock = errors.New("no clock to wait on")

	// ErrCalcRecursionLimit is returned when calc invocation nests deeper than
	// the run's calc depth budget, which an unbounded recursion would otherwise
	// do until the process ran out of stack.
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
	ErrCyclicSlot = errors.New("cyclic feature value dependency")

	// ErrConnectorEnd is returned when a connector cannot be attached to the
	// features its ends name: an end naming nothing reachable from the object
	// owning the connector, or one carrying no value. A connector whose ends
	// cannot be attached relates nothing, so it is an error rather than an object
	// with defaults at its ends.
	ErrConnectorEnd = errors.New("connector end cannot be attached")

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

	// ErrOutputNotAssigned is returned when a declared output carries no value
	// because the activation never assigned it. It is a kind of ErrNoValue.
	ErrOutputNotAssigned = fmt.Errorf("%w: output never assigned", ErrNoValue)

	// ErrConflictingOutput is returned when one activation would bind an output
	// twice: by its declaration and by an assignment, or by two assignments.
	ErrConflictingOutput = errors.New("output bound more than once")

	// ErrCyclicOutput is returned when an output feature's binding depends,
	// directly or through other outputs, on the output being computed.
	ErrCyclicOutput = errors.New("cyclic output dependency")

	// ErrAmbiguousResult is returned when a calc declaring several output
	// features is invoked as an expression. A function invocation has exactly
	// one result (KerML 7.4.9), so a calc that designates none has no value to
	// hand back and is read through a calc usage's output features instead.
	ErrAmbiguousResult = errors.New("calculation has no single result")

	// ErrIndexOutOfRange is returned when an index names no position of the
	// sequence or string it indexes; indices are 1-based, so 0 is out of range
	// as much as size+1 is, and each operation names what it indexed.
	ErrIndexOutOfRange = errors.New("index out of range")

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

	// ErrNotALiteral is returned when a name qualified by an enumeration
	// definition names something the enumeration does not declare as a literal.
	ErrNotALiteral = errors.New("not a literal of the enumeration")

	// ErrConflictingRedefinition is returned when one declaration values the
	// same feature under two of its names: a redefinition renames one feature,
	// so which of the two values it holds would be a silent pick.
	ErrConflictingRedefinition = errors.New("one feature valued under two names")

	// ErrValuedFeatureRestated is returned when a feature is both bound to a
	// value and given a body restating features of it: the bound value supplies
	// those features, so the restatement could only be silently dropped.
	ErrValuedFeatureRestated = errors.New("feature both valued and restated in a body")

	// ErrSlotMaterialization marks an error as a slot that could not be
	// materialized, whatever kept it from materializing. Reading a slot is what
	// finds such a failure, so a surface reporting one answered nothing about
	// that slot rather than deciding anything about the model.
	ErrSlotMaterialization = errors.New("feature value could not be materialized")

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

// SlotError marks a slot that could not be materialized. It reads as the error
// that kept the slot from materializing and unwraps to it as well as to
// ErrSlotMaterialization, so a caller tests either.
type SlotError struct {
	Err error
}

func (e *SlotError) Error() string { return e.Err.Error() }

func (e *SlotError) Unwrap() []error { return []error{ErrSlotMaterialization, e.Err} }

// OperandTypeError reports an operator applied to operand types it is not
// defined for, naming the operator and both operands and carrying the span of
// the expression so a surface holding the source can point at it.
type OperandTypeError struct {
	Op    string      // the operator, as written
	Left  string      // description of the left operand's type
	Right string      // description of the right operand's type
	Span  source.Span // span of the operator expression
}

func (e *OperandTypeError) Error() string {
	return fmt.Sprintf("%v: operator '%s' is not defined for %s and %s", ErrTypeMismatch, e.Op, e.Left, e.Right)
}

func (e *OperandTypeError) Unwrap() error { return ErrTypeMismatch }

// CalcFrameError reports an error raised inside a calc invocation, counting the
// calc frames it propagated through so a recursion reports a depth rather than
// one wrapped line per frame.
type CalcFrameError struct {
	Calc   string // the calc the error surfaced from
	Frames int    // calc frames the error propagated through
	Err    error

	// calcs names every calc the chain already passed through, so a cycle is
	// counted rather than wrapped again.
	calcs map[string]bool
}

func (e *CalcFrameError) Error() string {
	if e.Frames > 1 {
		return fmt.Sprintf("calc %s: … %d frames: %v", e.Calc, e.Frames, e.Err)
	}
	return fmt.Sprintf("calc %s: %v", e.Calc, e.Err)
}

func (e *CalcFrameError) Unwrap() error { return e.Err }

// calcFrame adds one calc frame to err. A calc the chain already passed through
// is counted rather than wrapped again, so a recursion reports a depth instead
// of one line per frame, while a calc calling another still names both.
func calcFrame(calc string, err error) error {
	var framed *CalcFrameError
	if errors.As(err, &framed) {
		if framed.calcs[calc] {
			return &CalcFrameError{
				Calc:   calc,
				Frames: framed.Frames + 1,
				Err:    framed.Err,
				calcs:  framed.calcs,
			}
		}
		calcs := make(map[string]bool, len(framed.calcs)+1)
		for name := range framed.calcs {
			calcs[name] = true
		}
		calcs[calc] = true
		return &CalcFrameError{Calc: calc, Frames: 1, Err: err, calcs: calcs}
	}
	return &CalcFrameError{Calc: calc, Frames: 1, Err: err, calcs: map[string]bool{calc: true}}
}

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
