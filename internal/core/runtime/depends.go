package runtime

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// exprRefs lists the names an expression reads, each with the feature chain it
// is a member segment of, so it is resolved the way evaluating it resolves it.
// It reports false for a node kind it does not know: an expression whose reads
// are unknown cannot be taken to have none.
func exprRefs(n ast.Node) ([]resolve.Reference, bool) {
	var refs []resolve.Reference
	ok := collectRefs(n, nil, &refs)
	return refs, ok
}

func collectRefs(n ast.Node, chain *ast.FeatureChainExpr, out *[]resolve.Reference) bool {
	if n == nil {
		return true
	}
	switch n := n.(type) {
	case *ast.LiteralBool, *ast.LiteralString, *ast.LiteralInteger, *ast.LiteralReal,
		*ast.LiteralInfinity, *ast.NullExpr, *ast.ErrorNode:
		return true
	case *ast.QualifiedName:
		*out = append(*out, resolve.Reference{QN: n, Chain: chain})
		return true
	case *ast.FeatureReference:
		return collectRefs(n.Name, chain, out)
	case *ast.OperatorExpr:
		ok := collectRefs(n.TypeRef, nil, out)
		for _, operand := range n.Operands {
			ok = collectRefs(operand, nil, out) && ok
		}
		return ok
	case *ast.FeatureChainExpr:
		// The member is a member of the operand, not of the scope the chain is
		// written in, so it is resolved through the chain it belongs to.
		return collectRefs(n.Operand, nil, out) && collectRefs(n.Member, n, out)
	case *ast.IndexExpr:
		return collectRefs(n.Operand, nil, out) && collectRefs(n.Index, nil, out)
	case *ast.InvocationExpr:
		ok := collectRefs(n.Operand, nil, out) && collectRefs(n.Type, nil, out)
		for _, arg := range n.Args {
			ok = collectRefs(arg, nil, out) && ok
		}
		// A named argument's name is a parameter of the invoked type, which is
		// recorded through the type itself.
		for _, arg := range n.NamedArgs {
			ok = collectRefs(arg.Value, nil, out) && ok
		}
		return ok
	case *ast.CollectExpr:
		return collectRefs(n.Operand, nil, out) && collectRefs(n.Body, nil, out)
	case *ast.SelectExpr:
		return collectRefs(n.Operand, nil, out) && collectRefs(n.Body, nil, out)
	case *ast.ConstructorExpr:
		ok := collectRefs(n.Type, nil, out)
		for _, arg := range n.Args {
			ok = collectRefs(arg, nil, out) && ok
		}
		return ok
	case *ast.BodyExpr:
		ok := collectRefs(n.Result, nil, out)
		for i := range n.Params {
			ok = collectRefs(n.Params[i].Type, nil, out) && ok
			ok = collectRefs(n.Params[i].Value, nil, out) && ok
		}
		return ok
	case *ast.SequenceExpr:
		ok := true
		for _, elem := range n.Elements {
			ok = collectRefs(elem, nil, out) && ok
		}
		return ok
	case *ast.MetadataAccessExpr:
		return collectRefs(n.Ref, nil, out)
	case *ast.CastExpr:
		return collectRefs(n.TargetType, nil, out)
	default:
		return false
	}
}

// recordReads records the declarations a feature's default expression reads, so
// a value computed from one of them is invalidated by a change to it rather than
// carried over as if it were still current. A name this context does not resolve
// read nothing here, and the declaration it would name — were it declared later
// — is not one this value came from.
func (ctx *Context) recordReads(feat *EffectiveFeature, shapes *Shapes) {
	if feat.DefaultValue == nil {
		return
	}
	refs, known := exprRefs(feat.DefaultValue)
	if !known {
		shapes.opaque = true
		return
	}
	scope := feat.DefaultScope()
	if scope == nil || ctx.resolver == nil {
		return
	}
	for _, ref := range refs {
		ref.Scope = scope
		if sym, ok := ctx.resolver.ResolveReference(ref); ok {
			ctx.recordReached(sym, shapes)
		}
	}
}

// recordReached records sym's shape and the declarations reachable from it: the
// types of its features and what their defaults read. A change to any of them is
// a change to what an object of sym holds.
func (ctx *Context) recordReached(sym *symbols.Symbol, shapes *Shapes) {
	fqn := ctx.fqnOf(sym)
	if fqn == "" {
		return
	}
	if _, done := shapes.digests[fqn]; done {
		return
	}
	shapes.digests[fqn] = ctx.ShapeDigest(sym)
	shapes.types = append(shapes.types, fqn)
	features := ctx.FeaturesOf(sym)
	for i := range features {
		ctx.recordReached(features[i].Type, shapes)
		ctx.recordReads(&features[i], shapes)
	}
}
