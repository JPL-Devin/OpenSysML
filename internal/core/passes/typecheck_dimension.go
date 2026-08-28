package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkDimensions warns when an operator that requires commensurable operands
// combines quantities of statically known, incommensurable dimensions —
// `mass < 1000.0[m]` — which evaluation rejects with an incommensurable-units
// error. It stays silent whenever either dimension is not statically determined,
// so a unit that only evaluation knows is never guessed at.
func (ec *exprChecker) checkDimensions(scope *symbols.Scope, e *ast.OperatorExpr) {
	if ec.model == nil || !commensurabilityRequired(e.Operator) || len(e.Operands) != 2 {
		return
	}
	lhs, ok := ec.model.DimensionOfExpr(scope, e.Operands[0])
	if !ok {
		return
	}
	rhs, ok := ec.model.DimensionOfExpr(scope, e.Operands[1])
	if !ok {
		return
	}
	if lhs.Term.Commensurable(rhs.Term) {
		return
	}
	ec.warnf(e.Span(), "operator '%s' combines incommensurable quantities: %s and %s",
		e.Operator, describeDimension(lhs), describeDimension(rhs))
}

// checkValueDimension reports a bound value measured in a dimension the
// target's declared quantity value type does not measure in — a speed bound to
// a DurationValue, whose mRef the libraries narrow to DurationUnit. A target
// declaring no quantity kind, and a value stating no measurement, are silent.
func (ec *exprChecker) checkValueDimension(valueScope, declScope *symbols.Scope, u *ast.Usage, value ast.Node) {
	if ec.model == nil {
		return
	}
	declared := ec.declaredTypeSymbol(declScope, u.Relationships)
	if declared == nil {
		return
	}
	want, ok := ec.model.DimensionOfType(declared)
	if !ok {
		return
	}
	for _, element := range valueElements(value) {
		if ec.judgedByType(valueScope, element) {
			// A named value is judged against the target by specialization,
			// which reports the same mismatch as a clash of types.
			continue
		}
		if statesNoMeasurement(element) {
			continue
		}
		got, ok := ec.model.DimensionOfExpr(valueScope, element)
		if !ok || want.Term.Commensurable(got.Term) {
			continue
		}
		ec.errorf(element.Span(), "cannot bind %s to a feature typed by %s",
			describeDimension(got), describeDimension(want))
	}
}

// judgedByType reports whether value conformance already types the element,
// which it does for a name and for an invocation of a behavior.
func (ec *exprChecker) judgedByType(scope *symbols.Scope, element ast.Node) bool {
	return ec.valueTypeSymbol(scope, element) != nil || ec.invocationResultTypeSymbol(scope, element) != nil
}

// statesNoMeasurement reports an expression written out of plain numbers alone:
// it names no unit, so it is read in the unit its target fixes rather than
// judged against it.
func statesNoMeasurement(element ast.Node) bool {
	switch n := element.(type) {
	case *ast.LiteralInteger, *ast.LiteralReal:
		return true
	case *ast.OperatorExpr:
		for _, operand := range n.Operands {
			if !statesNoMeasurement(operand) {
				return false
			}
		}
		return len(n.Operands) > 0
	}
	return false
}

// commensurabilityRequired reports whether an operator only relates operands of
// one dimension. A product or quotient combines dimensions instead, and a
// conditional does not relate its branches to the condition.
func commensurabilityRequired(op ast.OperatorKind) bool {
	switch op {
	case ast.OpAdd, ast.OpSub, ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe, ast.OpEq, ast.OpNeq:
		return true
	default:
		return false
	}
}

// describeDimension names the unit an operand was written in and the dimension it
// measures in, so the message says both what was written and why it clashes.
func describeDimension(d semantics.Dimension) string {
	if d.Term.Dimensionless() {
		if d.Unit == "" {
			return "a dimensionless value"
		}
		return fmt.Sprintf("%s (dimensionless)", d.Unit)
	}
	if d.Unit == "" {
		return fmt.Sprintf("a value of dimension %s", d)
	}
	return fmt.Sprintf("%s (dimension %s)", d.Unit, d)
}
