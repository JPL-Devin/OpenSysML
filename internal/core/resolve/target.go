package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ResolveTarget resolves a relationship target to the feature it names. A
// target is either a qualified name (`references takePicture`) or a feature
// chain (`perform providePower.generateTorque`), and the chain's segments are
// members of the previous segment rather than of the enclosing scope
// (SysML 7.6.6).
//
// Unlike resolveFeatureChain, this reports nothing: the target of a declared
// relationship is already resolved — and diagnosed — while walking the
// document, and this is the query side of that result.
func (r *Resolver) ResolveTarget(scope *symbols.Scope, target ast.Node) (*symbols.Symbol, bool) {
	switch t := target.(type) {
	case nil:
		return nil, false
	case *ast.QualifiedName:
		return r.ResolveQualified(scope, t)
	case *ast.FeatureReference:
		if t.Name == nil {
			return nil, false
		}
		return r.ResolveQualified(scope, t.Name)
	case *ast.FeatureChainExpr:
		owner, ok := r.ResolveTarget(scope, t.Operand)
		if !ok || t.Member == nil {
			return nil, false
		}
		return r.memberChain(owner, t.Member)
	default:
		return nil, false
	}
}

// ReferenceScope returns the scope a reference subsetting owned by decl
// resolves its target in. An unnamed feature takes its name from the feature it
// references (KerML Feature::effectiveName), so `perform 'provide power';`
// binds the name `'provide power'` in the very scope the target is looked up
// in. The target names the outer feature, not the reference to it, so a scope
// is skipped only when every symbol it binds under that name is decl itself —
// a sibling declaration of the same name is a legitimate target.
func ReferenceScope(scope *symbols.Scope, decl ast.Node, target ast.Node) *symbols.Scope {
	first := firstSegment(target)
	if first == "" || decl == nil {
		return scope
	}
	for s := scope; s != nil; s = s.Parent() {
		bound := s.LookupLocalAll(first)
		if len(bound) == 0 {
			return s
		}
		for _, sym := range bound {
			if sym.Decl != decl {
				return s
			}
		}
	}
	return scope
}

// ResolveReferenceTarget resolves the target of a reference subsetting owned by
// decl, which is declared in scope.
func (r *Resolver) ResolveReferenceTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool) {
	return r.ResolveTarget(ReferenceScope(scope, decl, target), target)
}

// firstSegment returns the leading name segment of a relationship target.
func firstSegment(target ast.Node) string {
	for {
		chain, ok := target.(*ast.FeatureChainExpr)
		if !ok {
			break
		}
		target = chain.Operand
	}
	qname := ast.AsQualifiedName(target)
	if qname == nil || len(qname.Parts) == 0 {
		return ""
	}
	return qname.Parts[0].Text
}

// memberChain walks qn's segments as members of owner, following inheritance
// where a semantic model is attached.
func (r *Resolver) memberChain(owner *symbols.Symbol, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	cur := owner
	for _, part := range qn.Parts {
		next, ok := r.lookupMember(cur, part.Text)
		if !ok && cur.Scope != nil {
			next, ok = cur.Scope.LookupLocal(part.Text)
		}
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}
