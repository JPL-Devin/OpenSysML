package passes

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
