package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// secondFQN names the unit the machine's virtual clock counts in, so a duration
// carrying a unit is expressed in seconds before it is scheduled.
const secondFQN = "SI::s"

// timeMagnitude reads a time trigger's duration or instant as a number of clock
// units: a bare number is already one, a quantity is converted from its unit.
func (e *StateExecutor) timeMagnitude(val Value, what string) (float64, error) {
	switch val.Kind {
	case ValConst:
		switch val.Const.Kind {
		case semantics.ValInt:
			return float64(val.Const.Int), nil
		case semantics.ValReal:
			return val.Const.Real, nil
		default:
			return 0, fmt.Errorf("%s must be numeric, got %v", what, val.Const.Kind)
		}
	case ValQuantity:
		return e.durationInClockUnits(val.Quantity(), what)
	default:
		return 0, fmt.Errorf("%s must be constant, got %v", what, val.Kind)
	}
}

// checkTimeTriggerType refuses, before evaluating it, the trigger argument
// validation refuses; one the declarations leave open is left to its value.
// The verdict is static, so it is judged once per transition and then reused.
func (e *StateExecutor) checkTimeTriggerType(trans *lower.Transition, t *ast.TimeEvent) error {
	if err, ok := e.timeTriggerVerdict[trans]; ok {
		return err
	}
	err := e.judgeTimeTriggerType(trans.Scope, t)
	e.timeTriggerVerdict[trans] = err
	return err
}

// judgeTimeTriggerType is the uncached judgement checkTimeTriggerType records.
func (e *StateExecutor) judgeTimeTriggerType(scope *symbols.Scope, t *ast.TimeEvent) error {
	c := e.ctx.model.TimeEventConforms(scope, t)
	if !c.Known || c.Holds {
		return nil
	}
	keyword := "after"
	if t.Absolute {
		keyword = "at"
	}
	return fmt.Errorf("%w: `%s %s` must be a %s, found %s",
		ErrTimeTriggerType, keyword, e.ctx.bindingExprText(t.Duration, scope), semantics.TimeEventType(t), c.Found)
}

// durationInClockUnits expresses a quantity in the clock's unit, reporting a
// quantity that does not measure time as the dimension error it is.
func (e *StateExecutor) durationInClockUnits(q *Quantity, what string) (float64, error) {
	second, err := e.clockUnit()
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", what, q, err)
	}
	// The dimension is reported as the error it is, keeping the sentinel a caller
	// can recognise; any other conversion failure is passed on as it was raised.
	if !q.Unit.Term.Commensurable(second.Term) {
		return 0, fmt.Errorf("%w: %s %s is not a time: %s does not measure a duration",
			ErrIncommensurableUnits, what, q, q.Unit)
	}
	magnitude, err := q.ConvertTo(second)
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", what, q, err)
	}
	return magnitude, nil
}

// clockUnit is the second as the Quantities and Units library reduces it, so a
// duration converts by the same reduction every other quantity uses.
func (e *StateExecutor) clockUnit() (Unit, error) {
	if e.ctx == nil || e.ctx.resolver == nil || e.ctx.resolver.Index() == nil {
		return Unit{}, fmt.Errorf("%w: no library to reduce %s in", semantics.ErrNotAUnit, secondFQN)
	}
	matches := e.ctx.resolver.Index().LookupQualified(secondFQN)
	if len(matches) != 1 {
		return Unit{}, fmt.Errorf("%w: %s names %d elements, so no clock unit is determined",
			semantics.ErrNotAUnit, secondFQN, len(matches))
	}
	term, err := e.ctx.model.UnitTermOf(matches[0])
	if err != nil {
		return Unit{}, err
	}
	return Unit{Text: "s", Product: semantics.NamedUnitProduct(matches[0], "s", false), Term: term}, nil
}
