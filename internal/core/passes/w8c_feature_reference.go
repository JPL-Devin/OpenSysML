package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateFeatureReferenceExpressionReferentIsFeature and
// validateFeatureChainExpressionConformance message.
const msgReferentIsFeature = "Must be a valid feature"

// FeatureReferencePass checks the features an expression names: a referent must
// be a feature (KerML 8.4.4.6.2/8.4.4.7.2), and a featured one must be reachable
// from where it is named (8.3.4.5.2 validateSubsettingFeaturingTypes).
type FeatureReferencePass struct{}

func (FeatureReferencePass) Level() PassLevel { return LevelConstraint }

func (FeatureReferencePass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &featureReferenceChecker{cc: &constraintChecker{
		model:    ctx.Model(),
		resolver: ctx.Resolver(),
	}}
	w := &w8cWalker{seen: map[*symbols.Symbol]bool{}}
	w.walk(rootScope, c.checkSymbol)
	return c.diags
}

// Operators taking a type, not a feature, as an operand.
var w8cTypeOperators = map[ast.OperatorKind]bool{
	ast.OpMeta:    true,
	ast.OpAll:     true,
	ast.OpAs:      true,
	ast.OpIsType:  true,
	ast.OpHasType: true,
	ast.OpAt:      true,
	ast.OpMetaAt:  true,
}

// w8cOwnsVariants reports whether sym's members are variants (enumeration
// literals included), which are owned rather than featured.
func w8cOwnsVariants(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.IsVariation || d.Kind == ast.DefEnumeration
	case *ast.Usage:
		return d.IsVariation || d.Kind == ast.UsageEnumeration
	}
	return false
}

type featureReferenceChecker struct {
	cc    *constraintChecker
	diags []Diagnostic
}

func (c *featureReferenceChecker) checkSymbol(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil {
		return
	}
	scope := sym.OwnerScope
	if sym.Scope != nil {
		scope = sym.Scope
	}
	c.walkExpr(sym, scope, u.Value)
}

// walkExpr visits the references an expression names. Nested declarations are
// symbols of their own and are visited as such.
func (c *featureReferenceChecker) walkExpr(sym *symbols.Symbol, scope *symbols.Scope, n ast.Node) {
	switch e := n.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		c.checkReferent(sym, scope, e, e.Span())
	case *ast.FeatureChainExpr:
		c.walkExpr(sym, scope, e.Operand)
		span := e.Span()
		if e.Member != nil {
			span = e.Member.Span()
		}
		c.checkReferent(sym, scope, e, span)
	case *ast.OperatorExpr:
		// These operators name types rather than features.
		if w8cTypeOperators[e.Operator] {
			return
		}
		for _, o := range e.Operands {
			c.walkExpr(sym, scope, o)
		}
	case *ast.IndexExpr:
		c.walkExpr(sym, scope, e.Operand)
		c.walkExpr(sym, scope, e.Index)
	case *ast.InvocationExpr:
		// An invocation names its function, and may pass one as an argument,
		// so neither position is a feature reference.
	case *ast.ConstructorExpr:
		for _, a := range e.Args {
			c.walkExpr(sym, scope, a)
		}
	case *ast.SequenceExpr:
		for _, el := range e.Elements {
			c.walkExpr(sym, scope, el)
		}
	}
}

// checkReferent reports a referent that is not a feature, or a feature that the
// naming context cannot reach.
func (c *featureReferenceChecker) checkReferent(sym *symbols.Symbol, scope *symbols.Scope, ref ast.Node, span source.Span) {
	chain, isChain := ref.(*ast.FeatureChainExpr)
	target, ok := c.cc.resolver.ResolveTarget(scope, ref)
	if !ok || target == nil || target == sym {
		return
	}
	if !isUsageKind(target.Kind) {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityError,
			Span:     span,
			Message:  msgReferentIsFeature,
			Code:     "feature-reference-referent",
			Source:   "constraint",
		})
		return
	}
	// A single-segment chain member is a feature of the preceding one, so the
	// naming context need not reach it; a qualified member names it through its
	// own namespace and is checked like any other reference.
	if isChain && (chain.Member == nil || len(chain.Member.Parts) < 2) {
		return
	}
	// A variant is an owned member of its variation, not a feature of it, so
	// it carries no featuring type to be accessible from.
	if tu, ok := target.Decl.(*ast.Usage); ok && tu.IsVariant {
		return
	}
	// A feature with no featuring type is accessible everywhere, so only a
	// featured one is checked.
	ctxs := c.cc.featuringContexts(target)
	if len(ctxs) == 0 {
		return
	}
	for _, ctx := range ctxs {
		if w8cOwnsVariants(ctx) {
			return
		}
	}
	if c.cc.redefinedAccessible(sym, target, map[*symbols.Symbol]bool{}) {
		return
	}
	// A feature of an implicit node (an accept parameter's action) has no
	// nameable dot path, and our scoping shares it with sibling nodes (W6C row
	// ~952, a deliberate divergence from the reference).
	if w8cOwnedByImplicitNode(target) {
		return
	}
	msg, code := msgSubsettingFeaturingTypes, "feature-reference-featuring-types"
	if isChain {
		msg, code = msgReferentIsFeature, "feature-reference-referent"
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msg,
		Code:     code,
		Source:   "constraint",
	})
}

// w8cOwnedByImplicitNode reports whether target is owned by an unnamed usage,
// which the parser creates for an implicit node such as an accept action.
func w8cOwnedByImplicitNode(target *symbols.Symbol) bool {
	if target == nil || target.OwnerScope == nil {
		return false
	}
	owner := target.OwnerScope.Owner()
	return owner != nil && owner.Name == "" && isUsageKind(owner.Kind)
}
