package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// endRedefinitionLookup is the part of the semantic model that reports the ends
// a connector or association end implicitly redefines, which a specializing
// association's ends never spell out (KerML 8.4.4.6).
type endRedefinitionLookup interface {
	ImplicitEndRedefinitions(sym *symbols.Symbol) []*symbols.Symbol
}

// nestedInRedefined resolves name among the features nested, at any depth, under
// a feature that an enclosing feature redefines (KerML 8.3.4.4). Reached only
// once ordinary lookup fails, so it shadows nothing.
func (r *Resolver) nestedInRedefined(scope *symbols.Scope, name string, hide *refFilter) (*symbols.Symbol, bool) {
	if name == "" {
		return nil, false
	}
	for s := scope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == nil || !isFeatureDecl(owner.Decl) {
			continue
		}
		for _, redefined := range r.redefinedFeatures(owner) {
			if sym, ok := r.nestedMember(redefined, name, hide); ok {
				return sym, true
			}
		}
	}
	return nil, false
}

// redefinedFeatures returns the features sym redefines, explicitly, implicitly
// in a metadata body, or as an association or connector end.
func (r *Resolver) redefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if cached, done := r.redefined[sym]; done {
		return cached
	}
	r.redefined[sym] = nil
	var out []*symbols.Symbol
	for _, rel := range redefinesRelationships(sym.Decl) {
		// Redefinitions search features of the owner's generals; hide only the
		// declaration's own binding so a same-named target reaches that feature.
		hide := &refFilter{decl: sym.Decl, skipBorrowedName: true}
		if found, ok := r.resolveTarget(sym.OwnerScope, rel.Target, hide); ok && found != sym {
			out = append(out, found)
		}
	}
	if model, ok := r.model.(endRedefinitionLookup); ok {
		for _, end := range model.ImplicitEndRedefinitions(sym) {
			if end != nil && end != sym {
				out = append(out, end)
			}
		}
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && sym.OwnerScope != nil &&
		sym.OwnerScope.BodyLocal() {
		if owner := sym.OwnerScope.Owner(); owner != nil {
			if target := symbols.MetadataBodyTarget(r.model, owner, usage.Ident); target != nil &&
				target != sym {
				out = append(out, target)
			}
		}
	}
	r.redefined[sym] = out
	return out
}

// nestedMember resolves name among the features nested under sym, at any depth,
// skipping what hide covers.
func (r *Resolver) nestedMember(sym *symbols.Symbol, name string, hide *refFilter) (*symbols.Symbol, bool) {
	seen := map[*symbols.Symbol]bool{}
	queue := []*symbols.Symbol{sym}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || seen[cur] || cur.Scope == nil {
			continue
		}
		seen[cur] = true
		for _, member := range cur.Scope.Members() {
			if member.Name == name && !hide.hides(member) {
				return member, true
			}
			queue = append(queue, member)
		}
	}
	return nil, false
}

// redefinesRelationships returns decl's explicit redefinitions.
func redefinesRelationships(decl ast.Node) []*ast.Relationship {
	var rels []*ast.Relationship
	switch d := decl.(type) {
	case *ast.Usage:
		rels = d.Relationships
	case *ast.Definition:
		rels = d.Relationships
	default:
		return nil
	}
	var out []*ast.Relationship
	for _, rel := range rels {
		if rel != nil && rel.Kind == ast.RelRedefines {
			out = append(out, rel)
		}
	}
	return out
}

// isFeatureDecl reports whether decl declares a feature rather than a type.
func isFeatureDecl(decl ast.Node) bool {
	_, ok := decl.(*ast.Usage)
	return ok
}
