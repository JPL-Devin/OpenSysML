package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxEvaluableDepth bounds the walk, which follows the value of every feature it
// reads and so could otherwise follow a cycle.
const maxEvaluableDepth = 32

// ModelLevelEvaluable reports whether an expression written in scope can be
// evaluated from the model alone (KerML 1.0 §7.4.9,
// Expression::isModelLevelEvaluable).
func (m *Model) ModelLevelEvaluable(scope *symbols.Scope, expr ast.Node) bool {
	if m == nil {
		return true
	}
	return m.evaluable(scope, expr, 0)
}

func (m *Model) evaluable(scope *symbols.Scope, expr ast.Node, depth int) bool {
	if expr == nil {
		return true
	}
	if depth > maxEvaluableDepth {
		return false
	}
	switch e := expr.(type) {
	case *ast.LiteralBool, *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralString,
		*ast.LiteralInfinity, *ast.NullExpr, *ast.MetadataAccessExpr:
		return true
	case *ast.SequenceExpr:
		return m.allEvaluable(scope, e.Elements, depth)
	case *ast.OperatorExpr:
		return m.evaluableOperator(scope, e, depth)
	case *ast.ConstructorExpr:
		// A constructor names a type and fills its features: the model decides it.
		return m.allEvaluable(scope, e.Args, depth)
	case *ast.InvocationExpr:
		return m.evaluableInvocation(scope, e, depth)
	case *ast.FeatureReference:
		return m.evaluableRead(scope, e.Name, depth)
	case *ast.IndexExpr:
		return m.evaluable(scope, e.Operand, depth+1) && m.evaluable(scope, e.Index, depth+1)
	case *ast.CollectExpr:
		return m.evaluable(scope, e.Operand, depth+1)
	case *ast.SelectExpr:
		return m.evaluable(scope, e.Operand, depth+1)
	case *ast.FeatureChainExpr:
		return m.evaluable(scope, e.Operand, depth+1)
	}
	// A cast, a body and anything else read the instance the expression runs on.
	return false
}

func (m *Model) allEvaluable(scope *symbols.Scope, nodes []ast.Node, depth int) bool {
	for _, n := range nodes {
		if !m.evaluable(scope, n, depth+1) {
			return false
		}
	}
	return true
}

// evaluableOperator decides an operator application. An operation over constants
// must fold: `~3` names only the abstract library function `DataFunctions::'~'`,
// which no concrete function implements for an Integer.
func (m *Model) evaluableOperator(scope *symbols.Scope, e *ast.OperatorExpr, depth int) bool {
	if !m.allEvaluable(scope, e.Operands, depth) {
		return false
	}
	if !allConstant(e.Operands) {
		return true
	}
	_, ok := evalConst(e)
	return ok
}

// allConstant reports whether every operand is a constant the model folds, so
// that failing to fold the operation is a statement about the operation.
func allConstant(nodes []ast.Node) bool {
	if len(nodes) == 0 {
		return false
	}
	for _, n := range nodes {
		if _, ok := evalConst(n); !ok {
			return false
		}
	}
	return true
}

// evaluableInvocation decides an invocation: the model evaluates a library
// function, never one the model under validation declares.
func (m *Model) evaluableInvocation(scope *symbols.Scope, e *ast.InvocationExpr, depth int) bool {
	if !m.evaluable(scope, e.Operand, depth+1) || !m.allEvaluable(scope, e.Args, depth) {
		return false
	}
	for _, arg := range e.NamedArgs {
		if !m.evaluable(scope, arg.Value, depth+1) {
			return false
		}
	}
	fn, ok := m.resolveExprTarget(scope, e.Type)
	return ok && m.libraryElement(fn)
}

// evaluableRead decides a feature read: naming a type or an enumeration literal
// is model-level, and so is naming a feature whose value is evaluable.
func (m *Model) evaluableRead(scope *symbols.Scope, qn *ast.QualifiedName, depth int) bool {
	sym, ok := m.resolveExprTarget(scope, qn)
	if !ok {
		return false
	}
	if EnumerationOwning(sym) != nil {
		return true
	}
	if usage, isUsage := sym.Decl.(*ast.Usage); isUsage && !isKerMLTypeDecl(sym) {
		return usage.Value != nil && m.evaluable(sym.OwnerScope, usage.Value, depth+1)
	}
	// Anything else named here is a type or a namespace, which the model holds.
	return true
}

// resolveExprTarget resolves a name an expression reads, following an alias.
func (m *Model) resolveExprTarget(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qn == nil {
		return nil, false
	}
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return nil, false
	}
	if target, aliasOK := m.resolver.ResolveAliasTarget(sym); aliasOK {
		sym = target
	}
	return sym, true
}

// libraryElement reports whether sym is declared by the bundled normative
// library, whose functions are the ones a model-level evaluation may call.
func (m *Model) libraryElement(sym *symbols.Symbol) bool {
	if sym == nil || m.resolver == nil {
		return false
	}
	idx := m.resolver.Index()
	return idx != nil && idx.Library(sym)
}
