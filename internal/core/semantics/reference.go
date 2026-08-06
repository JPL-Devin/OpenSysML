package semantics

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ReferencedFeature returns the feature sym reference-subsets: the target of
// the single `references` / `::>` edge its declaration may carry (KerML
// 8.3.3.3.9, "A Feature can have at most one ownedReferenceSubsetting"), or
// nil when it has none or the target does not resolve.
//
// The `perform` and `event` shorthands declare that edge without the keyword:
// `perform providePower.generateTorque` is a perform action usage whose
// performed action is related to it by reference subsetting (SysML 7.17.6).
//
// The result is memoized. Resolution of the target runs member lookup, which
// consults this relation in turn, so a symbol already being resolved yields nil
// rather than recursing.
func (m *Model) ReferencedFeature(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.referenced[sym]; ok {
		return cached
	}
	if m.resolvingRef[sym] {
		return nil
	}
	m.resolvingRef[sym] = true
	defer delete(m.resolvingRef, sym)

	var out *symbols.Symbol
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
			continue
		}
		target, ok := m.resolver.ResolveReferenceTarget(sym.OwnerScope, sym.Decl, rel.Target)
		if !ok || target == sym {
			continue
		}
		out = target
		break
	}
	m.referenced[sym] = out
	return out
}

// MemberSources returns the symbols whose scopes contribute members to sym, in
// deterministic breadth-first order and excluding sym itself: what sym
// specializes (DirectSupertypes) and what it reference-subsets
// (ReferencedFeature), transitively.
//
// Reference subsetting is a kind of subsetting, and subsetting is a kind of
// specialization (KerML 8.3.3.3.9), so a referencing feature inherits the
// referenced feature's features: `perform action takePhoto references
// takePicture` makes takePicture's members reachable as `takePhoto.focus`.
// It is kept out of DirectSupertypes because that relation also drives
// conformance and implicit typing, which this implementation does not yet
// derive from reference subsetting — see docs/SPEC_COMPLIANCE.md.
func (m *Model) MemberSources(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.memberSources[sym]; ok {
		return cached
	}

	var order []*symbols.Symbol
	visited := map[*symbols.Symbol]bool{sym: true}
	queue := m.contributors(sym)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || visited[cur] {
			continue
		}
		visited[cur] = true
		order = append(order, cur)
		queue = append(queue, m.contributors(cur)...)
	}
	// A query made while a reference target is being resolved sees that
	// reference as absent (the cycle guard above), so its result is only
	// provisional and is not cached.
	if len(m.resolvingRef) == 0 {
		m.memberSources[sym] = order
	}
	return order
}

// contributors returns the direct member-contributing neighbours of sym:
// its supertypes, then the feature it reference-subsets.
func (m *Model) contributors(sym *symbols.Symbol) []*symbols.Symbol {
	supers := m.DirectSupertypes(sym)
	ref := m.ReferencedFeature(sym)
	if ref == nil {
		return supers
	}
	out := make([]*symbols.Symbol, 0, len(supers)+1)
	out = append(out, supers...)
	return append(out, ref)
}
