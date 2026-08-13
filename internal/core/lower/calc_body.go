package lower

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// CalcBody lowers the computation a calculation body states, in declaration
// order and in the scope it was written in. A member stating no computation —
// an input parameter, documentation, a nested definition — is skipped.
// A result the body declares names the value answered with, not a step, so it
// runs after the steps; a `return` inside a branch or loop stops the body there.
func CalcBody(members []ast.Node, scope *symbols.Scope) []Statement {
	body := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if actual := unwrapMembership(member); actual != nil {
			body = append(body, actual)
		}
	}

	var stmts, results []Statement
	for _, member := range body {
		switch m := member.(type) {
		case *ast.Usage:
			if m.Direction == ast.DirIn || m.Direction == ast.DirInOut {
				// An input parameter is bound by the invocation, not by the body.
				continue
			}
			stmt, ok := usageStatement(m, scope)
			if !ok {
				continue
			}
			if _, isResult := stmt.(Return); isResult {
				results = append(results, stmt)
				continue
			}
			stmts = append(stmts, stmt)
		case *ast.Definition, *ast.Documentation, *ast.Comment, *ast.Import:
			// Declares a member of the calculation, not a step of it.
		case *ast.ResultMember:
			results = append(results, Return{Value: m.Expression, Node: m, Scope: scope})
		default:
			if isExpressionNode(member) {
				results = append(results, Return{Value: member, Node: member, Scope: scope})
				continue
			}
			stmts = append(stmts, lowerStatement(member, scope))
		}
	}
	return append(stmts, results...)
}

// usageStatement lowers a usage written in a statement position: a bound result
// parameter returns the value it binds, an accept parameter states an effect,
// and an attribute declares a value the statements around it read and write.
func usageStatement(u *ast.Usage, scope *symbols.Scope) (Statement, bool) {
	if u.IsAccept {
		return Effect{Kind: EffectAccept, Node: u, Scope: scope}, true
	}
	// A result parameter binding no value only names the result, so it states no
	// step of the computation.
	if u.IsResult || u.Direction == ast.DirOut {
		if u.Value == nil {
			return nil, false
		}
		return Return{Value: u.Value, Node: u, Scope: scope}, true
	}
	if name, _ := ast.EffectiveName(u); u.Kind == ast.UsageAttribute && name != "" {
		return Declare{Name: name, Value: u.Value, Node: u, Scope: scope}, true
	}
	return nil, false
}

// Returns reports whether the statements return a value on some path, so a
// calculation whose body only computes can be told from one that has no result.
func Returns(stmts []Statement) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case Return:
			return true
		case If:
			if Returns(s.Then.Statements) {
				return true
			}
			if s.Else != nil && Returns(s.Else.Statements) {
				return true
			}
		case Loop:
			if Returns(s.Body.Statements) {
				return true
			}
		}
	}
	return false
}

// isExpressionNode reports whether a body member is an expression rather than a
// declaration or a statement.
func isExpressionNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralBool, *ast.LiteralString,
		*ast.NullExpr, *ast.FeatureReference, *ast.FeatureChainExpr, *ast.OperatorExpr,
		*ast.SequenceExpr, *ast.CollectExpr, *ast.SelectExpr, *ast.InvocationExpr,
		*ast.IndexExpr, *ast.BodyExpr:
		return true
	default:
		return false
	}
}
