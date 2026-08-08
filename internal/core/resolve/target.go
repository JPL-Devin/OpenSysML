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
	return r.resolveTarget(scope, target, nil)
}

// resolveTarget is ResolveTarget with an optional reference filter, which
// applies to the leading segment of the target only: the rest of a feature
// chain is looked up in the preceding segment, not in the enclosing scope.
func (r *Resolver) resolveTarget(scope *symbols.Scope, target ast.Node, hide *refFilter) (*symbols.Symbol, bool) {
	switch t := target.(type) {
	case nil:
		return nil, false
	case *ast.QualifiedName:
		return r.resolveQualified(scope, t, hide)
	case *ast.FeatureReference:
		if t.Name == nil {
			return nil, false
		}
		return r.resolveQualified(scope, t.Name, hide)
	case *ast.FeatureChainExpr:
		owner, ok := r.resolveTarget(scope, t.Operand, hide)
		if !ok || t.Member == nil {
			return nil, false
		}
		return r.memberChain(owner, t.Member)
	default:
		return nil, false
	}
}

// refFilter hides, during one reference-subsetting resolution, the bindings the
// referring declaration itself contributes. An unnamed feature takes its name
// from the feature it references (KerML Feature::effectiveName), so
// `perform 'provide power';` binds `'provide power'` in the very scope its
// target is looked up in; the target names the referenced feature, never a
// reference to it, so such borrowed bindings are invisible to it while a
// declaration of that name — local, inherited or imported — is a valid target.
type refFilter struct{ decl ast.Node }

// hides reports whether sym is a binding a reference target must not see.
func (f *refFilter) hides(sym *symbols.Symbol) bool {
	if f == nil || sym == nil {
		return false
	}
	return sym.Decl == f.decl || sym.EffectiveName
}

// lookupLocal is Scope.LookupLocal with the hidden bindings removed. A nil
// filter hides nothing.
func (f *refFilter) lookupLocal(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	if scope == nil {
		return nil, false
	}
	if f == nil {
		return scope.LookupLocal(name)
	}
	visible := make([]*symbols.Symbol, 0, 1)
	for _, sym := range scope.LookupLocalAll(name) {
		if !f.hides(sym) {
			visible = append(visible, sym)
		}
	}
	if len(visible) == 0 {
		return nil, false
	}
	return symbols.PreferDeclared(visible)[0], true
}

// ResolveReferenceTarget resolves the target of a reference subsetting owned by
// decl, which is declared in scope.
func (r *Resolver) ResolveReferenceTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool) {
	return r.resolveTarget(scope, target, &refFilter{decl: decl})
}

// Reference describes one occurrence of a name to resolve on its own, outside a
// document walk: which scope it is written in, the declaration that refers to it
// when it is a reference subsetting's target, and the feature chain it is the
// member segment of. Used by the editor layer, which resolves a single name
// under the cursor and must reach the same symbol the document walk did.
type Reference struct {
	Scope *symbols.Scope
	QN    *ast.QualifiedName
	// Referrer owns the reference subsetting QN is the target of, if any.
	Referrer ast.Node
	// Chain is set when QN is the member of a feature chain, whose segments are
	// members of the operand rather than of Scope (SysML 7.6.6).
	Chain *ast.FeatureChainExpr
	// Redefines is set when QN is the target of a redefinition, which names a
	// feature of Scope's generals rather than a member of Scope itself.
	Redefines bool
}

// ResolveReference resolves a single name occurrence, honoring both the
// reference-subsetting and feature-chain rules.
func (r *Resolver) ResolveReference(ref Reference) (*symbols.Symbol, bool) {
	if ref.QN == nil {
		return nil, false
	}
	var hide *refFilter
	if ref.Referrer != nil {
		hide = &refFilter{decl: ref.Referrer}
	}
	if ref.Chain != nil {
		owner, ok := r.resolveTarget(ref.Scope, ref.Chain.Operand, hide)
		if !ok {
			return nil, false
		}
		return r.memberChain(owner, ref.QN)
	}
	if ref.Redefines {
		r.resolveRedefinition(ref.Scope, ref.QN, nil)
		return r.PartSymbol(ref.QN, len(ref.QN.Parts)-1)
	}
	return r.resolveQualified(ref.Scope, ref.QN, hide)
}

// leadingName returns the qualified name a relationship target starts with: the
// only part of it looked up in the enclosing scope.
func leadingName(target ast.Node) ast.Node {
	for {
		chain, ok := target.(*ast.FeatureChainExpr)
		if !ok {
			break
		}
		target = chain.Operand
	}
	if qname := ast.AsQualifiedName(target); qname != nil {
		return qname
	}
	return nil
}

// memberChain walks qn's segments as members of owner, following inheritance
// where a semantic model is attached.
func (r *Resolver) memberChain(owner *symbols.Symbol, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	cur := owner
	for i, part := range qn.Parts {
		next, ok := r.lookupMember(cur, part.Text)
		if !ok && cur.Scope != nil {
			next, ok = cur.Scope.LookupLocal(part.Text)
		}
		if !ok {
			return nil, false
		}
		cur = next
		r.recordPart(qn, i, cur)
	}
	return cur, true
}
