package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Diagnostic codes of the trigger argument rules (SysML v2 §8.3.17,
// TriggerInvocationExpression): the argument of `after` is a DurationValue,
// of `at` a TimeInstantValue, of `when` a Boolean.
const (
	codeTriggerAfterDuration = "trigger-after-duration"
	codeTriggerAtTimeInstant = "trigger-at-time-instant"
	codeTriggerWhenBoolean   = "trigger-when-boolean"

	msgTriggerAfterDuration = "an 'after' trigger's delay must be a DurationValue, found %s: write it with a duration unit (`after 5 [s]`) or name a feature typed by DurationValue"
	msgTriggerAtTimeInstant = "an 'at' trigger's time must be a TimeInstantValue, found %s: name a feature typed by TimeInstantValue or a clock reading such as `TimeOf(this)`"
	msgTriggerWhenBoolean   = "a 'when' trigger's condition must be Boolean, found %s: compare the value (`when x > 3`) or name a feature typed by Boolean"
)

// checkTrigger types a trigger. A change event carries a Boolean condition, a
// time event a duration (`after`) or a time instant (`at`); a signal or call
// trigger names an event and has nothing to type here. A payload parameter
// carries its trigger as its value. `transition ... when <expr>` leaves the
// trigger as a bare expression, a change-event condition unless it is a bare
// name — that names a signal.
func (ec *exprChecker) checkTrigger(scope *symbols.Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
	case *ast.ChangeEvent:
		ec.checkCondition(scope, t.Condition, codeTriggerWhenBoolean, msgTriggerWhenBoolean, true)
	case *ast.TimeEvent:
		ec.checkTimeEvent(scope, t)
	case *ast.Usage:
		if t.IsAccept && isTriggerValue(t.Value) {
			ec.checkTrigger(scope, t.Value)
		}
	case *ast.FeatureReference, *ast.QualifiedName, *ast.AcceptEvent, *ast.CallEvent:
	default:
		ec.checkCondition(scope, trigger, codeTriggerWhenBoolean, msgTriggerWhenBoolean, true)
	}
}

// isTriggerValue reports whether a usage's value is the trigger of an accept
// (`accept after 5 [s]`), checked by the body walk, rather than a bound value.
func isTriggerValue(value ast.Node) bool {
	switch value.(type) {
	case *ast.TimeEvent, *ast.ChangeEvent:
		return true
	}
	return false
}

// checkTimeEvent checks the argument of a time trigger against the library type
// it must be a value of. An argument whose type only evaluation determines is
// not reported.
func (ec *exprChecker) checkTimeEvent(scope *symbols.Scope, t *ast.TimeEvent) {
	if t.Duration == nil {
		return
	}
	ec.infer(scope, t.Duration)
	fqn, code, format := semantics.FQNDurationValue, codeTriggerAfterDuration, msgTriggerAfterDuration
	if t.Absolute {
		fqn, code, format = semantics.FQNTimeInstantValue, codeTriggerAtTimeInstant, msgTriggerAtTimeInstant
	}
	c := ec.model.ExprConformsToLibrary(scope, t.Duration, fqn)
	if !c.Known || c.Holds {
		return
	}
	ec.errorCode(code, t.Duration.Span(), format, c.Found)
}
