package resolve

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

		if sym, ok := r.acceptPayload(s, name); ok && !hide.hides(sym) {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.implicitlyNamedMember(s, name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.visibleMember(s.Owner(), name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.lookupInheritedImports(s, name); ok && !hide.hides(sym) {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := enclosingLocal(s.Parent(), name, hide); ok {
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
	if sym, ok := r.nestedInRedefined(scope, name, hide); ok {
		return resolution{sym: sym, ok: true}
	}
	// Final fallback: check global index (cross-document top-level names)
	if sym := r.lookupGlobalTop(scope, name); sym != nil && !hide.hides(sym) {
		return resolution{sym: sym, ok: true}
	}
	return resolution{}
}

func enclosingLocal(scope *symbols.Scope, name string, hide *refFilter) (*symbols.Symbol, bool) {
	for ; scope != nil; scope = scope.Parent() {
		if sym, ok := hide.lookupLocal(scope, name); ok {
			return sym, true
		}
	}
	return nil, false
}

// visibleMember resolves name as a member of sym, skipping what hide covers, so
// that a feature which borrowed a name does not mask the one it took it from.
func (r *Resolver) visibleMember(sym *symbols.Symbol, name string, hide *refFilter) (*symbols.Symbol, bool) {
	if hide.contributedOnly() {
		// The owner's own declarations are the local bindings already filtered
		// by the caller, so only contributed ones remain.
		found, ok := r.lookupContributedMember(sym, name)
		if !ok || !visibleAsInheritedMember(sym, found) ||
			r.inheritanceMaskedDeclaring(sym, found, declaredNameIn(sym, hide.decl)) {
			return nil, false
		}
		return found, true
	}
	found, ok := r.lookupMember(sym, name)
	if !ok || !visibleAsInheritedMember(sym, found) {
		return nil, false
	}
	// What a feature of sym redefines, sym does not inherit, so no name of it
	// resolves here (KerML 8.3.3.3); a redefinition being written is exempt.
	if r.inheritanceMaskedDeclaring(sym, found, declaredNameIn(sym, hide.declNode())) {
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
	for _, imp := range r.importsOf(node) {
		if r.resolvingImports[imp] {
			continue
		}
		if !r.importPrefixAvailable(scope, imp, name) {
			continue
		}
		if sym, ok := r.matchImport(scope, imp, name); ok {
			return sym, true
		}
	}
	return nil, false
}

// lookupImportedMember resolves a segment surfaced by the namespace being
// traversed, including a public membership import.
func (r *Resolver) lookupImportedMember(target *symbols.Symbol, targetScope, from *symbols.Scope, name string) (*symbols.Symbol, bool) {
	for _, imp := range r.importsOf(targetScope.Node()) {
		if imp.Kind != ast.ImportMembership {
			continue
		}
		if !r.importPrefixAvailable(targetScope, imp, name) {
			continue
		}
		if !r.importVisibleFrom(target, from, imp) {
			continue
		}
		if sym, ok := r.matchImport(targetScope, imp, name); ok {
			return sym, true
		}
	}
	return nil, false
}

func (r *Resolver) importVisibleFrom(target *symbols.Symbol, from *symbols.Scope, imp *ast.Import) bool {
	if imp.Visibility == ast.VisibilityProtected {
		var owner *symbols.Symbol
		if from != nil {
			owner = from.Owner()
		}
		return inheritedThroughSpecialization(imp) && r.specializes(owner, target)
	}
	if imp.Visibility != ast.VisibilityPrivate && !imp.IsExpose {
		return true
	}
	targetFQN := r.registeredFQN(target)
	fromFQN := r.ReferringNamespaceFQN(from)
	return targetFQN != "" && (fromFQN == targetFQN || strings.HasPrefix(fromFQN, targetFQN+"::"))
}

// importsOf is importsOf memoized: the tree is immutable once parsed, and every
// reference in a namespace looks its name up against that namespace's imports.
func (r *Resolver) importsOf(node ast.Node) []*ast.Import {
	if node == nil {
		return nil
	}
	if imports, ok := r.imports[node]; ok {
		return imports
	}
	imports := importsOf(node)
	r.imports[node] = imports
	return imports
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
//
// An element the import's filter clause rejects, or one rejected by a `filter`
// member of the namespace declaring the import, is not brought in at all — so a
// name only such an element bears does not resolve through this import
// (KerML 8.2.4, SysML v2 7.4.4). A rejected element does not end the search
// either: another element of the same name elsewhere in an imported subtree may
// be admitted.
func (r *Resolver) matchImport(scope *symbols.Scope, imp *ast.Import, name string) (*symbols.Symbol, bool) {
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return nil, false
	}
	if r.resolvingImports[imp] {
		return nil, false
	}
	r.resolvingImports[imp] = true
	// Resolved aside: a miss here may only mean sibling imports were suspended
	// for cycle safety, so it must not be memoized or reported as unresolved.
	var target *symbols.Symbol
	var ok bool
	r.aside(func() { target, ok = r.resolveImportTarget(scope, imp) })
	delete(r.resolvingImports, imp)
	if !ok {
		return nil, false
	}
	admit := r.importAdmits(scope, imp)
	if imp.Kind == ast.ImportMembership {
		// A membership import names a membership: `import P::Car` where Car is an
		// alias imports that name, so the alias is what it surfaces.
		if alias, isAlias := r.PartAlias(imp.Imported, len(imp.Imported.Parts)-1); isAlias {
			target = alias
		}
		// The imported member itself (last segment) is visible by its own name.
		// For FQN-indexed symbols (stdlib), target.Name may be the full FQN, so extract last segment.
		targetName := target.Name
		if idx := strings.LastIndex(targetName, "::"); idx >= 0 {
			targetName = targetName[idx+2:]
		}
		if (targetName == name || (target.ShortName != "" && target.ShortName == name)) && admit(target) {
			return target, true
		}
		if imp.IsRecursive && target.Scope != nil {
			if sym, ok := lookupInSubtree(target.Scope, name, imp, admit, map[*symbols.Scope]bool{}); ok {
				return sym, true
			}
		}
		return nil, false
	}
	// Namespace import: visible members of the target's scope are surfaced.
	// Check scope first if available
	if target.Scope != nil {
		if sym, ok := target.Scope.LookupLocal(name); ok && visibleThroughImport(imp, sym) && admit(sym) {
			return sym, true
		}
		if sym, ok := r.lookupImportedMember(target, target.Scope, scope, name); ok && symbols.VisibleOutside(sym.Visibility) && admit(sym) {
			return sym, true
		}
	}
	// Also check FQN index for re-exported symbols (wildcard imports populate index, not scope)
	if r.idx != nil {
		// A name only a private import surfaced under the target is a member of
		// it but not a visible one, so a wildcard import does not re-export it
		// (KerML 8.2.3.3) — unless this is an `import all`, which takes the
		// target's private memberships too.
		targetFQN := r.registeredFQN(target)
		var children []*symbols.Symbol
		if importAllowsPrivate(imp) {
			children = r.idx.LookupDirectChildren(targetFQN)
		} else {
			children = r.idx.LookupDirectChildrenFrom(targetFQN, r.ReferringNamespaceFQN(scope))
		}
		// The import names one namespace, so a same-named other namespace's
		// members registered under the same path are not what it surfaces.
		children = notConflatedWith(target, children)
		for _, sym := range children {
			// Extract short name from FQN for comparison
			symName := sym.Name
			if idx := strings.LastIndex(symName, "::"); idx >= 0 {
				symName = symName[idx+2:]
			}
			if symName != name && (sym.ShortName == "" || sym.ShortName != name) {
				continue
			}
			// The target may itself have surfaced the name through an import of
			// its own, filtered by its `filter` members: what it re-exports
			// onward is what those select.
			if !r.admitsUnderName("", r.ReferringNamespaceFQN(scope), targetFQN+"::"+name, sym) {
				continue
			}
			if r.importSurfaces(imp, targetFQN, sym) && admit(sym) {
				return sym, true
			}
		}
	}
	if imp.IsRecursive {
		if sym, ok := lookupInSubtree(target.Scope, name, imp, admit, map[*symbols.Scope]bool{}); ok {
			return sym, true
		}
	}
	return nil, false
}

func (r *Resolver) importPrefixAvailable(scope *symbols.Scope, imp *ast.Import, name string) bool {
	if len(r.resolvingImports) == 0 || imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return true
	}
	// During recursive import resolution, avoid re-entering a path with no independent prefix binding.
	prefix := imp.Imported.Parts[0].Text
	if prefix == name {
		return true
	}
	for s := scope; s != nil; s = s.Parent() {
		if _, ok := s.LookupLocal(prefix); ok {
			return true
		}
	}
	if r.idx != nil && len(r.idx.LookupQualified(prefix)) > 0 {
		return true
	}
	return false
}

// lookupInSubtree searches a scope and all descendant scopes for a match on
// name that imp may surface. A match the import cannot surface does not end the
// walk: another scope in the subtree may hold a visible one.
func lookupInSubtree(scope *symbols.Scope, name string, imp *ast.Import, admit func(*symbols.Symbol) bool, seen map[*symbols.Scope]bool) (*symbols.Symbol, bool) {
	// A body-local name is not a member of the namespace being imported.
	if scope == nil || seen[scope] || scope.BodyLocal() {
		return nil, false
	}
	seen[scope] = true
	if sym, ok := scope.LookupLocal(name); ok && visibleThroughImport(imp, sym) && admit(sym) {
		return sym, true
	}
	for _, child := range scope.Children() {
		// A recursive import descends through the memberships it can see, so a
		// namespace it cannot surface hides its own contents too.
		if owner := child.Owner(); owner != nil && !visibleThroughImport(imp, owner) {
			continue
		}
		if sym, ok := lookupInSubtree(child, name, imp, admit, seen); ok {
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
