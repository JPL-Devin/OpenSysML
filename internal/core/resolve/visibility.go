package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// importAllowsPrivate reports whether an import re-exports private members.
// Only `import all` widens visibility to include private members.
func importAllowsPrivate(imp *ast.Import) bool {
	return imp.IsAll
}

// visibleThroughImport reports whether sym may be surfaced by imp when
// enumerating a namespace's members. Private members are hidden unless the
// import is `import all`.
func visibleThroughImport(imp *ast.Import, sym *symbols.Symbol) bool {
	if sym.Visibility == ast.VisibilityPrivate {
		return importAllowsPrivate(imp)
	}
	return true
}

// inheritedThroughSpecialization reports whether imp reaches the bodies that
// specialize the definition or usage declaring it: a protected or public one
// does (SysML v2 7.5.3), a private membership does not (KerML 8.2.3.3).
func inheritedThroughSpecialization(imp *ast.Import) bool {
	return imp.Visibility != ast.VisibilityPrivate
}

func (r *Resolver) specializes(from, target *symbols.Symbol) bool {
	if from == nil || target == nil {
		return false
	}
	if from == target {
		return true
	}
	for _, sup := range r.specializationChain(from) {
		if sup == target {
			return true
		}
	}
	return false
}

func (r *Resolver) specializationChain(from *symbols.Symbol) []*symbols.Symbol {
	if from == nil {
		return nil
	}
	model, ok := r.model.(supertypeLookup)
	if !ok {
		return nil
	}
	seen := map[*symbols.Symbol]bool{from: true}
	queue := model.DirectSupertypes(from)
	var chain []*symbols.Symbol
	for len(queue) > 0 {
		sup := queue[0]
		queue = queue[1:]
		if sup == nil || seen[sup] {
			continue
		}
		seen[sup] = true
		chain = append(chain, sup)
		queue = append(queue, model.DirectSupertypes(sup)...)
	}
	return chain
}

// lookupInheritedImports resolves name through the imports declared by what
// scope's owner specializes, walking the specialization graph upward: inside
// `part def Sub :> Base` it consults Base's protected imports. A feature typing
// is a generalization edge (KerML 8.3.4.6), so `part p : Base` reaches them too.
func (r *Resolver) lookupInheritedImports(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	owner := scope.Owner()
	if owner == nil || r.inheritedImports[owner] {
		return nil, false
	}
	_, ok := r.model.(supertypeLookup)
	if !ok {
		return nil, false
	}
	// Resolving an inherited import's target resolves names again, which may
	// arrive back here: one visit per owner per lookup.
	r.inheritedImports[owner] = true
	defer delete(r.inheritedImports, owner)

	for _, sup := range r.specializationChain(owner) {
		if sup.Scope != nil {
			for _, imp := range importsOf(sup.Scope.Node()) {
				if !inheritedThroughSpecialization(imp) {
					continue
				}
				if !r.importPrefixAvailable(sup.Scope, imp, name) {
					continue
				}
				if sym, ok := r.matchImport(sup.Scope, imp, name); ok {
					return sym, true
				}
			}
		}
	}
	return nil, false
}
