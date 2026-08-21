package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Redefinition masking (KerML 7.4.7, 8.3.3.3): a redefined feature is not
// inherited by the type owning the redefining feature, so none of its names
// are visible there. The mask is keyed by element, not by name, so a chain of
// redefinitions masks every link and an inherited namesake nobody redefines
// stays visible.

// RedefinedFeatures returns the features sym redefines: the resolved target of
// each explicit `redefines`/`:>>` clause. Alias targets are resolved through, so
// the result names elements rather than the bindings that reach them. The
// implicit redefinitions of parameters and connector ends are matched by
// position rather than declared, and are reported by DirectSupertypes only.
// Memoized.
func (m *Model) RedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	if cached, ok := m.redefined[sym]; ok {
		return cached
	}
	m.redefined[sym] = nil // re-entrancy guard for cyclic declarations

	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	add := func(target *symbols.Symbol) {
		if target == nil || target == sym || seen[target] {
			return
		}
		seen[target] = true
		out = append(out, target)
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		add(m.redefinitionTarget(sym, rel.Target))
	}

	m.redefined[sym] = out
	return out
}

// DeclaresRedefinition reports whether sym carries a `redefines`/`:>>` clause,
// whether or not its target resolves.
func DeclaresRedefinition(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}

// redefinitionTarget resolves one redefinition target reference. A single-segment
// name denotes the feature the owner inherits under it, not the declaration that
// borrowed the name in the owner's own scope (KerML 7.3.4.5).
func (m *Model) redefinitionTarget(sym *symbols.Symbol, target ast.Node) *symbols.Symbol {
	if ref, ok := target.(*ast.FeatureReference); ok {
		target = ref.Name
	}
	switch node := target.(type) {
	case *ast.QualifiedName:
		if len(node.Parts) == 1 {
			if inherited := m.inheritedFeature(sym, node); inherited != nil {
				// An alias names its target: what is redefined is the element.
				if resolved, aliasOK := m.resolver.ResolveAliasTarget(inherited); aliasOK {
					return resolved
				}
				return inherited
			}
		}
		found, ok := m.resolver.ResolveQualified(sym.OwnerScope, node)
		if !ok || found == nil {
			return nil
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(found); aliasOK {
			return resolved
		}
	case *ast.FeatureChainExpr:
		if found, ok := m.resolver.ResolveTarget(sym.OwnerScope, node); ok {
			return found
		}
	}
	return nil
}

// redefinitionMask returns the elements sym does not inherit because a feature
// of sym — owned or itself inherited — redefines them. Members sym declares are
// never masked: only inheritance is affected. When declared is false the
// features sym declares mask nothing, which is how a declaration written in sym
// sees what sym inherits (KerML 8.3.3.3.6). Memoized.
func (m *Model) redefinitionMask(sym *symbols.Symbol, declared bool) map[*symbols.Symbol]bool {
	if m == nil || sym == nil {
		return nil
	}
	cache := m.redefMask
	if !declared {
		cache = m.redefMaskInherited
	}
	if cached, ok := cache[sym]; ok {
		return cached
	}
	cache[sym] = nil // re-entrancy guard: a nested query sees no mask

	mask := make(map[*symbols.Symbol]bool)
	pending := m.maskCandidates(sym, declared)
	for i := 0; i < len(pending); i++ {
		redefining := pending[i]
		for _, target := range m.RedefinedFeatures(redefining) {
			if target == sym || mask[target] || sharesName(redefining, target) {
				continue
			}
			mask[target] = true
			pending = append(pending, target) // a redefined feature's own targets are masked too
		}
	}
	// A local declaration is present whatever it redefines.
	if sym.Scope != nil {
		for _, local := range sym.Scope.AllMembers() {
			delete(mask, local)
		}
	}
	if len(mask) == 0 {
		mask = nil
	}
	cache[sym] = mask
	return mask
}

// maskCandidates returns the features whose redefinitions can mask something on
// sym: those it inherits or reference-subsets, plus, when declared, its own.
func (m *Model) maskCandidates(sym *symbols.Symbol, declared bool) []*symbols.Symbol {
	var out []*symbols.Symbol
	if declared && sym.Scope != nil {
		out = append(out, sym.Scope.AllMembers()...)
	}
	for _, src := range m.MemberSources(sym) {
		if src == nil || src.Scope == nil {
			continue
		}
		out = append(out, src.Scope.AllMembers()...)
	}
	return out
}

// sharesName reports whether a redefining feature is visible under a name of
// the feature it redefines — `feature :>> cyl` takes the name `cyl` (KerML
// 7.3.4.5). Such a target masks no name, and hiding it would break resolution
// of the very reference that names it.
func sharesName(redefining, target *symbols.Symbol) bool {
	if redefining == nil || target == nil {
		return false
	}
	for _, name := range []string{maskLeafName(redefining.Name), redefining.ShortName} {
		if name == "" {
			continue
		}
		if name == maskLeafName(target.Name) || name == target.ShortName {
			return true
		}
	}
	return false
}

// maskLeafName returns the last segment of a qualified name.
func maskLeafName(name string) string {
	if i := lastDoubleColon(name); i >= 0 {
		return name[i+2:]
	}
	return name
}

// InheritanceMasked reports whether sym does not inherit candidate because one
// of its features redefines it. Callers enumerating or resolving inherited
// members ask here; resolving a redefinition target itself must not, as that
// reference names the masked feature (KerML 7.3.4.5).
func (m *Model) InheritanceMasked(sym, candidate *symbols.Symbol) bool {
	return m.inheritanceMasked(sym, candidate, true)
}

func (m *Model) inheritanceMasked(sym, candidate *symbols.Symbol, declared bool) bool {
	mask := m.redefinitionMask(sym, declared)
	if len(mask) == 0 || candidate == nil {
		return false
	}
	if mask[candidate] {
		return true
	}
	target, ok := m.resolver.ResolveAliasTarget(candidate)
	return ok && mask[target]
}
