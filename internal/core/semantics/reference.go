package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
	if node := referenceSubsettingTarget(sym); node != nil {
		if target, ok := m.resolver.ResolveReferenceTarget(referenceScope(sym), sym.Decl, node); ok && target != sym {
			out = target
		}
	} else if u, isUsage := sym.Decl.(*ast.Usage); isUsage && u.IsVariant && u.Keyword == "variant" && sym.Name != "" {
		// A bare `variant X` is a VariantReference (SysML.xtext:642): a
		// reference usage subsetting the like-named feature visible outside.
		if named, ok := m.resolver.LookupNameExcluding(sym.OwnerScope, sym.Name, sym.Decl); ok && named != sym {
			out = named
		}
	}
	// A result computed while another symbol's reference is in flight saw a
	// truncated member view (that symbol's own reference was hidden), so it is
	// provisional and must not be cached.
	if len(m.resolvingRef) == 1 {
		m.referenced[sym] = out
	}
	return out
}

// referenceSubsettingTarget returns the node naming the feature sym
// reference-subsets, or nil when it has no such clause. A connector end carries
// that clause outside its relationship list when it is written with the
// `references` keyword, so it is asked for its own.
func referenceSubsettingTarget(sym *symbols.Symbol) ast.Node {
	if end, ok := sym.Decl.(*ast.ConnectorEnd); ok {
		return end.ReferencedTarget()
	}
	for _, rel := range RelationshipsOf(sym) {
		// `include 'add fuel'` is an OwnedReferenceSubsetting in the grammar
		// (SysML.xtext IncludeUseCaseUsage), so an inclusion contributes too.
		if rel != nil && (rel.Kind == ast.RelReferences || rel.Kind == ast.RelIncludes) && rel.Target != nil {
			return rel.Target
		}
	}
	return nil
}

// referenceScope returns the scope sym's reference-subsetting target is written
// in. A connector end is a member of the connector, but what it attaches to is a
// feature of the connector's owner, so it resolves one scope out — the same
// scope the document walk uses (resolve/document.go).
func referenceScope(sym *symbols.Symbol) *symbols.Scope {
	if _, ok := sym.Decl.(*ast.ConnectorEnd); ok && sym.OwnerScope != nil {
		return sym.OwnerScope.Parent()
	}
	return sym.OwnerScope
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
// derive from reference subsetting — see docs/project/spec-compliance.md.
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

// contributors returns the direct member-contributing neighbours of sym: its
// supertypes, the base usage every usage element subsets, then the feature it
// reference-subsets. The base usage contributes members only, not conformance.
func (m *Model) contributors(sym *symbols.Symbol) []*symbols.Symbol {
	supers := m.DirectSupertypes(sym)
	out := make([]*symbols.Symbol, 0, len(supers)+2)
	out = append(out, supers...)
	if base := m.implicitBaseUsage(sym); base != nil {
		out = append(out, base)
	}
	if ref := m.ReferencedFeature(sym); ref != nil {
		out = append(out, ref)
	}
	return out
}
