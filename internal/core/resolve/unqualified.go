package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// walkUnqualified searches the scope and its ancestors for a local match or an
// imported match, then falls back to the document root.
func (r *Resolver) walkUnqualified(scope *symbols.Scope, name string) resolution {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := s.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
		if sym, ok := r.lookupImports(s, name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	if root := rootOf(scope); root != nil {
		if sym, ok := root.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	return resolution{}
}

// lookupImports checks every import declared directly in scope for a member
// matching name.
func (r *Resolver) lookupImports(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	node := scope.Node()
	for _, imp := range importsOf(node) {
		if sym, ok := r.matchImport(scope, imp, name); ok {
			return sym, true
		}
	}
	return nil, false
}

// importsOf returns the *ast.Import declarations directly in a namespace-bearing node.
func importsOf(node ast.Node) []*ast.Import {
	var members []ast.Node
	switch n := node.(type) {
	case *ast.Package:
		members = n.Members
	case *ast.Namespace:
		members = n.Members
	case *ast.RootNamespace:
		members = n.Members
	default:
		return nil
	}
	var out []*ast.Import
	for _, m := range members {
		if imp, ok := m.(*ast.Import); ok {
			out = append(out, imp)
		}
	}
	return out
}

// matchImport tries to satisfy name through a single import declaration.
func (r *Resolver) matchImport(scope *symbols.Scope, imp *ast.Import, name string) (*symbols.Symbol, bool) {
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return nil, false
	}
	target, ok := r.ResolveQualified(scope, imp.Imported)
	if !ok {
		return nil, false
	}
	if imp.Kind == ast.ImportMembership {
		// The imported member itself (last segment) is visible by its own name.
		if target.Name == name {
			return target, true
		}
		if imp.IsRecursive && target.Scope != nil {
			if sym, ok := lookupInSubtree(target.Scope, name, map[*symbols.Scope]bool{}); ok {
				return sym, true
			}
		}
		return nil, false
	}
	// Namespace import: members of the target's scope are visible.
	if target.Scope == nil {
		return nil, false
	}
	if sym, ok := target.Scope.LookupLocal(name); ok {
		return sym, true
	}
	if imp.IsRecursive {
		if sym, ok := lookupInSubtree(target.Scope, name, map[*symbols.Scope]bool{}); ok {
			return sym, true
		}
	}
	return nil, false
}

// lookupInSubtree searches a scope and all descendant scopes for name.
func lookupInSubtree(scope *symbols.Scope, name string, seen map[*symbols.Scope]bool) (*symbols.Symbol, bool) {
	if scope == nil || seen[scope] {
		return nil, false
	}
	seen[scope] = true
	if sym, ok := scope.LookupLocal(name); ok {
		return sym, true
	}
	for _, child := range scope.Children() {
		if sym, ok := lookupInSubtree(child, name, seen); ok {
			return sym, true
		}
	}
	return nil, false
}
