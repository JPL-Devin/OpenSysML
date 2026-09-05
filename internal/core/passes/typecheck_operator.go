package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Codes and messages of the pilot's three operator-expression rules
// (validateOperatorExpression{BracketOperator,Quantity,CastConformance}).
const (
	codeBracketOperator = "bracket-operator"
	codeQuantityUnit    = "quantity-unit"
	codeCastConformance = "cast-conformance"

	msgBracketOperator = "`x[i]` is not an index in KerML: `[` invokes BaseFunctions::'[', which the kernel library leaves abstract; index a sequence with `x#(i)`"
	msgQuantityUnit    = "the unit of a quantity must be a measurement reference, found %s: write a unit such as `[m]` or name a feature typed by MeasurementUnit or another measurement reference"
	msgCastConformance = "cast argument is typed by %s, unrelated to the target %s: neither type specializes the other, so the cast selects no value"
)

// checkOperatorRules applies the three rules to every operator of an expression
// the scalar type checker does not walk: a filter condition or a multiplicity bound.
func (ec *exprChecker) checkOperatorRules(scope *symbols.Scope, node ast.Node) {
	switch e := node.(type) {
	case nil:
		return
	case *ast.OperatorExpr:
		if e.Operator == ast.OpAs {
			ec.checkCast(scope, e)
		}
		for _, o := range e.Operands {
			ec.checkOperatorRules(scope, o)
		}
	case *ast.IndexExpr:
		if e.Bracket {
			ec.checkBracket(scope, e)
		}
		ec.checkOperatorRules(scope, e.Operand)
		ec.checkOperatorRules(scope, e.Index)
	case *ast.FeatureChainExpr:
		ec.checkOperatorRules(scope, e.Operand)
	case *ast.InvocationExpr:
		ec.checkOperatorRules(scope, e.Operand)
		for _, a := range e.Args {
			ec.checkOperatorRules(scope, a)
		}
		for _, na := range e.NamedArgs {
			ec.checkOperatorRules(scope, na.Value)
		}
	case *ast.ConstructorExpr:
		for _, a := range e.Args {
			ec.checkOperatorRules(scope, a)
		}
		for _, na := range e.NamedArgs {
			ec.checkOperatorRules(scope, na.Value)
		}
	case *ast.SequenceExpr:
		for _, el := range e.Elements {
			ec.checkOperatorRules(scope, el)
		}
	case *ast.CollectExpr:
		ec.checkOperatorRules(scope, e.Operand)
		ec.checkOperatorRules(scope, e.Body)
	case *ast.SelectExpr:
		ec.checkOperatorRules(scope, e.Operand)
		ec.checkOperatorRules(scope, e.Body)
	case *ast.BodyExpr:
		for i := range e.Params {
			ec.checkOperatorRules(scope, e.Params[i].Value)
		}
		ec.checkOperatorRules(ec.bodyScope(scope, e), e.Result)
	}
}
