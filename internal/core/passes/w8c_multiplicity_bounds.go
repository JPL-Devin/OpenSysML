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
	w8cWalkSymbols(ctx, rootScope, c.check)
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
	if _, isInf := bound.(*ast.LiteralInfinity); isInf {
		return
	}
	prim := c.boundPrimType(scope, bound)
	if prim != semantics.PrimUnknown && !semantics.PrimConforms(prim, semantics.PrimInteger) {
		c.report(bound)
	}
}

// boundPrimType types a bound expression, reading through a feature the bound
// names to the value that feature is bound to.
func (c *multiplicityBoundsChecker) boundPrimType(scope *symbols.Scope, bound ast.Node) semantics.PrimType {
	silent := &exprChecker{resolver: c.resolver, model: c.model}
	if w8cIsReference(bound) {
		if sym, ok := c.resolver.ResolveTarget(scope, bound); ok && sym != nil {
			return silent.featurePrimType(sym)
		}
		return semantics.PrimUnknown
	}
	return silent.infer(scope, bound)
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
