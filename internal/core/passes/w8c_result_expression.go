package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateFunctionResultExpressionMembership and
// validateExpressionResultExpressionMembership message.
const msgResultExpressionAtMostOne = "Only one (owned or inherited) result expression is allowed"

// ResultExpressionPass checks that a function or expression has at most one
// result expression, owned or inherited (KerML 8.3.4.5.2). A function that
// inherits a result expression from a supertype cannot add its own, and one
// that inherits two along different specializations is invalid on its own.
type ResultExpressionPass struct{}

func (ResultExpressionPass) Level() PassLevel { return LevelConstraint }

func (ResultExpressionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &resultExpressionChecker{resolver: ctx.Resolver()}
	w := &w8cWalker{seen: make(map[*symbols.Symbol]bool)}
	w.walk(rootScope, c.check)
	return c.diags
}

type resultExpressionChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (c *resultExpressionChecker) check(sym *symbols.Symbol) {
	u, ok := w8cUsageOf(sym)
	if !ok || (u.Kind != ast.UsageCalc && u.Kind != ast.UsageExpr) {
		return
	}
	owned := w8cResultExpressions(u)
	inherited := c.inheritedResultExpressions(sym, map[*symbols.Symbol]bool{})
	if len(owned)+inherited < 2 {
		return
	}
	span := u.Span()
	if len(owned) > 0 {
		span = owned[0].Span()
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msgResultExpressionAtMostOne,
		Code:     "result-expression-at-most-one",
		Source:   "constraint",
	})
}

// inheritedResultExpressions counts the result expressions sym inherits along
// its typings and specializations, transitively.
func (c *resultExpressionChecker) inheritedResultExpressions(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) int {
	if sym == nil || seen[sym] {
		return 0
	}
	seen[sym] = true
	count := 0
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
		default:
			continue
		}
		target, ok := c.resolver.ResolveTarget(w8cScopeOf(sym), rel.Target)
		if !ok || target == nil || target == sym {
			continue
		}
		if u, ok := w8cUsageOf(target); ok {
			count += len(w8cResultExpressions(u))
		}
		count += c.inheritedResultExpressions(target, seen)
	}
	return count
}

// w8cResultExpressions returns the body expressions of a function or expression
// that act as its result: bare expression members, not nested declarations.
func w8cResultExpressions(u *ast.Usage) []ast.Node {
	var out []ast.Node
	for _, m := range u.Members {
		n := unwrapType(m)
		if w8cIsResultExpressionNode(n) {
			out = append(out, n)
		}
	}
	return out
}

func w8cIsResultExpressionNode(n ast.Node) bool {
	switch n.(type) {
	case *ast.Usage, *ast.Definition, *ast.Package, *ast.Namespace, *ast.Import,
		*ast.Membership, *ast.ResultMember, *ast.Comment, *ast.Documentation,
		*ast.ErrorNode, nil:
		return false
	default:
		return true
	}
}
