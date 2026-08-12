package resolve

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// walkUnqualified searches the scope and its ancestors for a local match or an
// imported match, then falls back to the document root and finally the global index.
func (r *Resolver) walkUnqualified(scope *symbols.Scope, name string) resolution {
	return r.walkUnqualifiedHiding(scope, name, nil)
}

// LookupName resolves an unqualified reference from scope the way a written one
// resolves — the enclosing scope chain, inherited members, imports, then the
// global index — but records no diagnostic when it finds nothing.
//
// It is what an evaluator asks: the names it looks up are not all references the
// source wrote down (a value bound at run time is looked up the same way), and a
// miss is for the evaluator to report against the value it was reading.
func (r *Resolver) LookupName(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	res := r.walkUnqualified(scope, name)
	return res.sym, res.ok
}

// walkUnqualifiedHiding is walkUnqualified with the bindings hide covers made
// invisible. Only those bindings are hidden: each scope's inherited members and
// imports are still consulted, so a perform statement's borrowed name does not
// hide the action its owner inherits from its type.
func (r *Resolver) walkUnqualifiedHiding(scope *symbols.Scope, name string, hide *refFilter) resolution {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := hide.lookupLocal(s, name); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.implicitlyNamedMember(s, name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.visibleMember(s.Owner(), name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.lookupImports(s, name); ok && !hide.hides(sym) {
			return resolution{sym: sym, ok: true}
		}
	}
	if root := rootOf(scope); root != nil {
		if sym, ok := hide.lookupLocal(root, name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	// Final fallback: check global index (cross-document top-level names)
	if sym, n := r.lookupGlobalTop(name); n == 1 && !hide.hides(sym) {
		return resolution{sym: sym, ok: true}
	}
	return resolution{}
}

// visibleMember resolves name as a member of sym, skipping what hide covers, so
// that a feature which borrowed a name does not mask the one it took it from.
func (r *Resolver) visibleMember(sym *symbols.Symbol, name string, hide *refFilter) (*symbols.Symbol, bool) {
	if hide.contributedOnly() {
		// The owner's own declarations are the local bindings already filtered
		// by the caller, so only contributed ones remain.
		return r.lookupContributedMember(sym, name)
	}
	found, ok := r.lookupMember(sym, name)
	if !ok {
		return nil, false
	}
	if !hide.hides(found) {
		return found, true
	}
	return r.lookupContributedMember(sym, name)
}

// implicitlyNamedMember returns the anonymous member of scope that binds name by
// implicitly redefining a parameter so called (KerML 7.3.4.5, SysML 7.6.5):
// `action shoot : Shoot { in item; }` binds `image`. That target is known only
// to the semantic model, hence the binding here and not when scopes are built.
func (r *Resolver) implicitlyNamedMember(scope *symbols.Scope, name string, hide *refFilter) (*symbols.Symbol, bool) {
	if scope == nil || name == "" || !scope.HasAnonymousMembers() {
		return nil, false
	}
	model, ok := r.model.(supertypeLookup)
	if !ok {
		return nil, false
	}
	for _, sym := range scope.AnonymousMembers() {
		if r.naming[sym] || hide.hides(sym) || !impliesNamingFeature(sym) {
			continue
		}
		r.naming[sym] = true
		var redefined *symbols.Symbol
		count := 0
		for _, sup := range model.DirectSupertypes(sym) {
			if isParameter(sup) {
				redefined = sup
				count++
			}
		}
		delete(r.naming, sym)
		// One redefinition names the feature; several need not agree on a
		// name, so the feature stays anonymous (KerML 7.3.4.5).
		if count == 1 && simpleName(redefined) == name {
			return sym, true
		}
	}
	return nil, false
}

// impliesNamingFeature reports whether sym is a nameless parameter whose naming
// feature is implicit: any declared relationship other than a typing would have
// named it, or ruled it out, when scopes were built.
func impliesNamingFeature(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !isParameter(sym) {
		return false
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind != ast.RelTyping {
			return false
		}
	}
	return true
}

// isParameter reports whether sym is declared as a directed feature or a
// result, the features an implicit redefinition matches (SysML 7.6.5).
func isParameter(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && (usage.Direction != ast.DirNone || usage.IsResult)
}

// simpleName is sym's own name without the qualification an indexed symbol
// carries.
func simpleName(sym *symbols.Symbol) string {
	if i := strings.LastIndex(sym.Name, "::"); i >= 0 {
		return sym.Name[i+2:]
	}
	return sym.Name
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

// importsOf returns the *ast.Import declarations directly in a namespace-bearing
// node. In KerML an Import is a Relationship owned by any Namespace, and a
// definition or usage body is itself a Namespace, so imports declared inside a
// definition/usage body apply to that body's scope (and, through the ordinary
// parent-scope walk, to every scope nested within it).
func importsOf(node ast.Node) []*ast.Import {
	var members []ast.Node
	switch n := node.(type) {
	case *ast.Package:
		members = n.Members
	case *ast.Namespace:
		members = n.Members
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	case *ast.Usage:
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
		// For FQN-indexed symbols (stdlib), target.Name may be the full FQN, so extract last segment.
		targetName := target.Name
		if idx := strings.LastIndex(targetName, "::"); idx >= 0 {
			targetName = targetName[idx+2:]
		}
		if targetName == name {
			return target, true
		}
		// Also check short name (e.g., "kg" for "kilogram")
		if target.ShortName != "" && target.ShortName == name {
			return target, true
		}
		if imp.IsRecursive && target.Scope != nil {
			if sym, ok := lookupInSubtree(target.Scope, name, imp, map[*symbols.Scope]bool{}); ok {
				return sym, true
			}
		}
		return nil, false
	}
	// Namespace import: visible members of the target's scope are surfaced.
	// Check scope first if available
	if target.Scope != nil {
		if sym, ok := target.Scope.LookupLocal(name); ok && visibleThroughImport(imp, sym) {
			return sym, true
		}
	}
	// Also check FQN index for re-exported symbols (wildcard imports populate index, not scope)
	if r.idx != nil {
		children := r.idx.LookupDirectChildren(target.Name)
		for _, sym := range children {
			// Extract short name from FQN for comparison
			symName := sym.Name
			if idx := strings.LastIndex(symName, "::"); idx >= 0 {
				symName = symName[idx+2:]
			}
			if symName == name && visibleThroughImport(imp, sym) {
				return sym, true
			}
			// Also check short name (e.g., "kg" for "kilogram")
			if sym.ShortName != "" && sym.ShortName == name && visibleThroughImport(imp, sym) {
				return sym, true
			}
		}
	}
	if imp.IsRecursive {
		if sym, ok := lookupInSubtree(target.Scope, name, imp, map[*symbols.Scope]bool{}); ok {
			return sym, true
		}
	}
	return nil, false
}

// lookupInSubtree searches a scope and all descendant scopes for a match on
// name that imp may surface. A match the import cannot surface does not end the
// walk: another scope in the subtree may hold a visible one.
func lookupInSubtree(scope *symbols.Scope, name string, imp *ast.Import, seen map[*symbols.Scope]bool) (*symbols.Symbol, bool) {
	// A body-local name is not a member of the namespace being imported.
	if scope == nil || seen[scope] || scope.BodyLocal() {
		return nil, false
	}
	seen[scope] = true
	if sym, ok := scope.LookupLocal(name); ok && visibleThroughImport(imp, sym) {
		return sym, true
	}
	for _, child := range scope.Children() {
		if sym, ok := lookupInSubtree(child, name, imp, seen); ok {
			return sym, true
		}
	}
	return nil, false
}

// LookupNameExcluding resolves name from scope as LookupName does, with decl's
// own binding hidden: what the name resolves to where decl does not declare it.
func (r *Resolver) LookupNameExcluding(scope *symbols.Scope, name string, decl ast.Node) (*symbols.Symbol, bool) {
	res := r.walkUnqualifiedHiding(scope, name, &refFilter{decl: decl})
	return res.sym, res.ok
}
