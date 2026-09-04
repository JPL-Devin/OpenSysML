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
	m.computingRedefinedFeatures++
	defer func() {
		m.computingRedefinedFeatures--
	}()

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

// AllRedefinedFeatures returns every feature sym redefines, directly or through
// the features those redefine: explicit clauses and the implicit redefinitions
// of parameters, connector ends and case roles alike. sym itself is excluded.
func (m *Model) AllRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{sym: true}
	var visit func(*symbols.Symbol)
	visit = func(s *symbols.Symbol) {
		for _, target := range m.directRedefinedFeatures(s) {
			if target == nil || seen[target] {
				continue
			}
			seen[target] = true
			out = append(out, target)
			visit(target)
		}
	}
	visit(sym)
	return out
}

// directRedefinedFeatures returns the features sym redefines by clause or by position.
func (m *Model) directRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	explicit := m.RedefinedFeatures(sym)
	out := make([]*symbols.Symbol, 0, len(explicit))
	out = append(out, explicit...)
	out = append(out, m.implicitParameterRedefinitions(sym)...)
	out = append(out, m.implicitEndRedefinitions(sym)...)
	return append(out, m.ImplicitRoleRedefinitions(sym)...)
}

// EffectiveNameOf returns a declaration's name, including one inherited through a unique redefinition.
func (m *Model) EffectiveNameOf(sym *symbols.Symbol) string {
	return m.effectiveNameOf(sym, make(map[*symbols.Symbol]bool))
}

func (m *Model) effectiveNameOf(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) string {
	if sym == nil || seen[sym] {
		return ""
	}
	if sym.Name != "" {
		return sym.Name
	}
	seen[sym] = true

	var targets []*symbols.Symbol
	known := make(map[*symbols.Symbol]bool)
	add := func(candidates []*symbols.Symbol) {
		for _, candidate := range candidates {
			if candidate == nil || candidate == sym || known[candidate] {
				continue
			}
			known[candidate] = true
			targets = append(targets, candidate)
		}
	}
	if m != nil {
		add(m.RedefinedFeatures(sym))
		add(m.implicitParameterRedefinitions(sym))
		add(m.implicitEndRedefinitions(sym))
		add(m.ImplicitRoleRedefinitions(sym))
	}
	if len(targets) != 1 {
		return ""
	}
	return m.effectiveNameOf(targets[0], seen)
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

// NotYetMember reports whether a member of a namespace a declaration is being
// written in is that declaration: the one identified, or, when the caller cannot
// tell which it is, any redefinition the namespace declares (KerML 8.3.3.3.6).
func NotYetMember(sym, declaring *symbols.Symbol) bool {
	if declaring != nil {
		return sym == declaring
	}
	return DeclaresRedefinition(sym)
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
	mask := m.buildMaskFromCandidates(sym, func(yield func(*symbols.Symbol) bool) {
		m.forEachMaskCandidate(sym, declared, yield)
	})
	cache[sym] = mask
	return mask
}

// redefinitionMaskExcluding is the mask on sym with one of its own declarations
// left out: the declaration being written, whose redefinition must not hide the
// feature it names (KerML 8.3.3.3.6). Not memoized — it is asked per edit.
func (m *Model) redefinitionMaskExcluding(sym, exclude *symbols.Symbol) map[*symbols.Symbol]bool {
	if m == nil || sym == nil {
		return nil
	}
	return m.buildMaskFromCandidates(sym, func(yield func(*symbols.Symbol) bool) {
		m.forEachMaskCandidate(sym, true, func(candidate *symbols.Symbol) bool {
			if candidate == exclude {
				return true
			}
			return yield(candidate)
		})
	})
}

// buildMask closes candidates' redefinitions transitively into the set of
// elements sym does not inherit.
func (m *Model) buildMask(sym *symbols.Symbol, candidates []*symbols.Symbol) map[*symbols.Symbol]bool {
	mask := make(map[*symbols.Symbol]bool)
	pending := candidates
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
		sym.Scope.ForEachMember(func(local *symbols.Symbol) bool {
			delete(mask, local)
			return true
		})
	}
	if len(mask) == 0 {
		return nil
	}
	return mask
}

// buildMaskFromCandidates unions cached candidate closures, falling back to the
// exact expansion when a closure is cyclic or reaches the owner.
func (m *Model) buildMaskFromCandidates(
	sym *symbols.Symbol,
	iterate func(func(*symbols.Symbol) bool),
) map[*symbols.Symbol]bool {
	mask := make(map[*symbols.Symbol]bool)
	fallback := false
	iterate(func(candidate *symbols.Symbol) bool {
		if candidate == nil {
			return true
		}
		closure, cyclic := m.redefinitionClosure(candidate)
		if cyclic || closure[sym] {
			fallback = true
			return false
		}
		for target := range closure {
			mask[target] = true
		}
		return true
	})
	if fallback {
		candidates := make([]*symbols.Symbol, 0, 8)
		iterate(func(candidate *symbols.Symbol) bool {
			candidates = append(candidates, candidate)
			return true
		})
		return m.buildMask(sym, candidates)
	}
	if sym.Scope != nil {
		sym.Scope.ForEachMember(func(local *symbols.Symbol) bool {
			delete(mask, local)
			return true
		})
	}
	if len(mask) == 0 {
		return nil
	}
	return mask
}

// redefinitionClosure returns the targets reached from candidate, excluding
// edges whose target keeps the redefining feature's visible name.
func (m *Model) redefinitionClosure(candidate *symbols.Symbol) (map[*symbols.Symbol]bool, bool) {
	if candidate == nil {
		return nil, false
	}
	if cached, ok := m.redefClosure[candidate]; ok {
		return cached, false
	}
	if m.computingRedefClosure[candidate] {
		return nil, true
	}
	m.computingRedefClosure[candidate] = true
	out := make(map[*symbols.Symbol]bool)
	cyclic := false
	for _, target := range m.RedefinedFeatures(candidate) {
		if sharesName(candidate, target) {
			continue
		}
		out[target] = true
		child, childCyclic := m.redefinitionClosure(target)
		if childCyclic {
			cyclic = true
		}
		for nested := range child {
			out[nested] = true
		}
	}
	delete(m.computingRedefClosure, candidate)
	if cyclic {
		return out, true
	}
	if m.computingRedefinedFeatures == 0 {
		m.redefClosure[candidate] = out
	}
	return out, false
}

// forEachMaskCandidate visits features whose redefinitions can mask something
// on sym, preserving declaration and inherited source order.
func (m *Model) forEachMaskCandidate(sym *symbols.Symbol, declared bool, yield func(*symbols.Symbol) bool) {
	if sym == nil || yield == nil {
		return
	}
	if declared && sym.Scope != nil {
		stopped := false
		sym.Scope.ForEachMember(func(candidate *symbols.Symbol) bool {
			if !yield(candidate) {
				stopped = true
				return false
			}
			return true
		})
		if stopped {
			return
		}
	}
	for _, src := range m.MemberSources(sym) {
		if src == nil || src.Scope == nil {
			continue
		}
		stopped := false
		src.Scope.ForEachMember(func(candidate *symbols.Symbol) bool {
			if !yield(candidate) {
				stopped = true
				return false
			}
			return true
		})
		if stopped {
			return
		}
	}
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
	return m.maskedBy(m.redefinitionMask(sym, true), candidate)
}

// InheritanceMaskedDeclaring is InheritanceMasked as the declaration named
// declName, being written in sym, sees it: sym's own redefinitions mask nothing,
// so the target such a declaration names stays resolvable (KerML 8.3.3.3.6).
func (m *Model) InheritanceMaskedDeclaring(sym, candidate *symbols.Symbol, declName string) bool {
	if declName == "" {
		return m.maskedBy(m.redefinitionMask(sym, false), candidate)
	}
	return m.maskedBy(m.declaringMask(sym, declName), candidate)
}

// declaringMask is the inherited mask on sym leaving out the redefinitions of
// features named declName: a declaration of that name redefines them, so its
// own redefinition governs what it can name (KerML 7.3.4.5). Memoized.
func (m *Model) declaringMask(sym *symbols.Symbol, declName string) map[*symbols.Symbol]bool {
	if m == nil || sym == nil {
		return nil
	}
	key := declMaskKey{owner: sym, name: declName}
	if cached, ok := m.declMask[key]; ok {
		return cached
	}
	m.declMask[key] = nil // re-entrancy guard: a nested query sees no mask
	mask := m.buildMaskFromCandidates(sym, func(yield func(*symbols.Symbol) bool) {
		m.forEachMaskCandidate(sym, false, func(candidate *symbols.Symbol) bool {
			if candidate == nil || maskLeafName(candidate.Name) == declName || candidate.ShortName == declName {
				return true
			}
			return yield(candidate)
		})
	})
	m.declMask[key] = mask
	return mask
}

// viewMask returns the elements sym does not inherit under the given view.
func (m *Model) viewMask(sym *symbols.Symbol, view memberView, declaring *symbols.Symbol) map[*symbols.Symbol]bool {
	switch {
	case view == memberViewUnmasked:
		return nil
	case view == memberViewDeclaring && declaring == nil:
		return m.redefinitionMask(sym, false)
	case view == memberViewDeclaring:
		return m.redefinitionMaskExcluding(sym, declaring)
	}
	return m.redefinitionMask(sym, true)
}

func (m *Model) maskedBy(mask map[*symbols.Symbol]bool, candidate *symbols.Symbol) bool {
	if len(mask) == 0 || candidate == nil {
		return false
	}
	if mask[candidate] {
		return true
	}
	target, ok := m.resolver.ResolveAliasTarget(candidate)
	return ok && mask[target]
}
