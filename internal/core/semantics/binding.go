package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// LookupBinding resolves the name a named argument binds within typ: a single segment
// is looked up as a member of typ, a qualified name from scope; aliases resolve through.
func (m *Model) LookupBinding(scope *symbols.Scope, typ *symbols.Symbol, name *ast.QualifiedName) (*symbols.Symbol, bool) {
	if m == nil || name == nil || len(name.Parts) == 0 {
		return nil, false
	}
	var (
		found *symbols.Symbol
		ok    bool
	)
	if len(name.Parts) == 1 {
		found, ok = m.LookupMember(typ, name.Parts[0].Text)
	} else {
		found, ok = m.resolver.ProbeReference(resolve.Reference{Scope: scope, QN: name})
	}
	if !ok || found == nil {
		return nil, false
	}
	if target, isAlias := m.resolver.ResolveAliasTarget(found); isAlias {
		return target, true
	}
	return found, true
}

// IsFeatureOf reports whether sym is a feature of typ: owned or inherited, and not
// redefined by another feature of typ.
func (m *Model) IsFeatureOf(typ, sym *symbols.Symbol) bool {
	if m == nil || typ == nil || sym == nil || !isFeature(sym) {
		return false
	}
	member := false
	for _, candidate := range m.MembersOf(typ) {
		if candidate == sym {
			member = true
			continue
		}
		if isFeature(candidate) && m.redefines(candidate, sym) {
			return false
		}
	}
	return member
}

// redefines reports whether sym redefines target, directly or through the
// features its targets redefine in turn.
func (m *Model) redefines(sym, target *symbols.Symbol) bool {
	visited := map[*symbols.Symbol]bool{sym: true}
	pending := []*symbols.Symbol{sym}
	for len(pending) > 0 {
		cur := pending[0]
		pending = pending[1:]
		for _, redefined := range m.RedefinedFeatures(cur) {
			if redefined == target {
				return true
			}
			if !visited[redefined] {
				visited[redefined] = true
				pending = append(pending, redefined)
			}
		}
	}
	return false
}
