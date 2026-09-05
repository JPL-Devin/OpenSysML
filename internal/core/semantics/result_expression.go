package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ResultExpression is a result expression a function or expression owns
// (KerML 7.4.7): the trailing expression of a calculation body or the bare
// condition of a constraint body.
type ResultExpression struct {
	Owner *symbols.Symbol
	Node  ast.Node
}

// FunctionLike reports whether sym declares a function or an expression, the
// types whose bodies state a result expression.
func FunctionLike(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if _, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return true
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefCalc, ast.DefConstraint, ast.DefRequirement, ast.DefConcern,
			ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase,
			ast.DefPredicate, ast.DefBool:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageCalc, ast.UsageExpr, ast.UsageConstraint, ast.UsageRequirement,
			ast.UsageConcern, ast.UsageFramedConcern, ast.UsageSatisfy, ast.UsageObjective,
			ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase,
			ast.UsagePredicate, ast.UsageBool:
			return true
		}
	}
	return false
}

// IsResultExpression reports whether a body member states its owner's result:
// a bare expression, or the bare condition of a constraint body. A nested
// assertion is a constraint of its own, not its owner's result.
func IsResultExpression(node ast.Node) bool {
	if c, ok := node.(*ast.ConstraintMember); ok {
		return c.Keyword == "" && c.Expression != nil
	}
	return ast.IsExpression(node)
}

// OwnedResultExpressions lists the result expressions sym's own body states,
// in declaration order.
func OwnedResultExpressions(sym *symbols.Symbol) []ResultExpression {
	if !FunctionLike(sym) {
		return nil
	}
	var out []ResultExpression
	for _, member := range declMembers(sym) {
		if m, ok := member.(*ast.Membership); ok {
			member = m.Member
		}
		if !IsResultExpression(member) {
			continue
		}
		node := member
		if c, ok := member.(*ast.ConstraintMember); ok {
			node = c.Expression
		}
		out = append(out, ResultExpression{Owner: sym, Node: node})
	}
	return out
}

// ResultExpressionsOf lists the result expressions sym owns or inherits, its
// own first and then those of every type contributing members to it —
// specialized or reference-subsetted — in breadth-first order (KerML 8.3.4.6
// Expression::result, 8.3.4.8 Function::result). Each source contributes once,
// however many paths reach it; a body listing several conditions contributes each.
func (m *Model) ResultExpressionsOf(sym *symbols.Symbol) []ResultExpression {
	if m == nil || !FunctionLike(sym) {
		return nil
	}
	out := OwnedResultExpressions(sym)
	for _, source := range m.MemberSources(sym) {
		out = append(out, OwnedResultExpressions(source)...)
	}
	return out
}

// ResultExpressionOwners lists the types whose bodies state the result
// expressions sym owns or inherits, sym itself first. A valid function or
// expression has at most one: a type that inherits a result states none.
func (m *Model) ResultExpressionOwners(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, expr := range m.ResultExpressionsOf(sym) {
		if len(out) == 0 || out[len(out)-1] != expr.Owner {
			out = append(out, expr.Owner)
		}
	}
	return out
}
