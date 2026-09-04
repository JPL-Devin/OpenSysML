package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateMultiplicityRangeBoundResultTypes message.
const msgMultiplicityBoundNatural = "Must have a Natural value"

// MultiplicityBoundsPass checks that every multiplicity bound has a Natural
// value (KerML 8.3.3.1.9, validateMultiplicityRangeBoundResultTypes): a
// model-level evaluable bound must evaluate to a non-negative whole number or
// `*`, and any other bound must have an Integer-conforming result type.
type MultiplicityBoundsPass struct{}

func (MultiplicityBoundsPass) Level() PassLevel { return LevelConstraint }

func (MultiplicityBoundsPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &multiplicityBoundsChecker{resolver: ctx.Resolver(), model: ctx.Model()}
	w := &w8cWalker{ctx: ctx}
	w.walk(rootScope, c.check)
	return c.diags
}

type multiplicityBoundsChecker struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	diags    []Diagnostic
}

func (c *multiplicityBoundsChecker) check(sym *symbols.Symbol) {
	mult := w8cMultiplicityOf(sym)
	if mult == nil {
		return
	}
	scope := w8cScopeOf(sym)
	c.checkBound(scope, mult.Lower)
	if mult.IsRange {
		c.checkBound(scope, mult.Upper)
	}
}

func (c *multiplicityBoundsChecker) checkBound(scope *symbols.Scope, bound ast.Node) {
	if bound == nil {
		return
	}
	if v, ok := w8cEvalConst(c.resolver, c.model, scope, bound, nil); ok {
		if v.Kind == semantics.ValInfinity || (v.Kind == semantics.ValInt && v.Int >= 0) {
			return
		}
		c.report(bound)
		return
	}
	if !c.boundIsInteger(scope, bound) {
		c.report(bound)
	}
}

// boundIsInteger reports whether a bound's result may be an Integer: a known
// result type must conform to Integer (a class-typed feature does not), while
// an unresolved or untyped bound is left to the tiers that own those gaps.
func (c *multiplicityBoundsChecker) boundIsInteger(scope *symbols.Scope, bound ast.Node) bool {
	switch e := bound.(type) {
	case *ast.LiteralInteger, *ast.LiteralInfinity:
		return true
	case *ast.OperatorExpr:
		if w8cIntegerOperator(e.Operator) {
			for _, arg := range e.Operands {
				if !c.boundIsInteger(scope, arg) {
					return false
				}
			}
			return true
		}
	}
	silent := &exprChecker{resolver: c.resolver, model: c.model}
	if !w8cIsReference(bound) {
		prim := silent.infer(scope, bound)
		return prim == semantics.PrimUnknown || semantics.PrimConforms(prim, semantics.PrimInteger)
	}
	sym, ok := c.resolver.ResolveTarget(scope, bound)
	if !ok || sym == nil {
		return true
	}
	if prim := silent.featurePrimType(sym); prim != semantics.PrimUnknown {
		return semantics.PrimConforms(prim, semantics.PrimInteger)
	}
	return !c.model.DeclaresResolvedType(sym)
}

// w8cIntegerOperator reports the arithmetic operators whose result is an
// Integer whenever every operand is one; division may yield a Rational.
func w8cIntegerOperator(op ast.OperatorKind) bool {
	switch op {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpMod, ast.OpPow, ast.OpNeg, ast.OpPos:
		return true
	}
	return false
}

func (c *multiplicityBoundsChecker) report(bound ast.Node) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     bound.Span(),
		Message:  msgMultiplicityBoundNatural,
		Code:     "multiplicity-bound-natural",
		Source:   "constraint",
	})
}
