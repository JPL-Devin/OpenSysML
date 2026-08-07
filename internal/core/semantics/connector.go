package semantics

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Implicit redefinition of connector ends by position (SysML v2 7.13.2, 7.14.2,
// KerML 7.4.6): an end of a connector, in lexical order, redefines the end at
// the same position of each connector its owner specializes, and so takes that
// end's type.

// connectorLike reports whether sym declares a connector — the only owning
// types whose end features are matched by position, and the only general types
// whose ends are implicitly redefined.
func connectorLike(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefConnection, ast.DefInterface, ast.DefAllocation, ast.DefFlow,
			ast.DefAssoc, ast.DefBinding:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation,
			ast.UsageConnector, ast.UsageFlow, ast.UsageSuccession,
			ast.UsageAssoc, ast.UsageBinding:
			return true
		}
	}
	return false
}

// ownedEnds returns the end features sym owns, one entry per end in
// declaration order: first the ends of its `connect` clause, then the `end`
// features of its body. An end that declares no name of its own — `connect a
// to b` — still occupies its position, as does one whose symbol is not
// registered; both are reported as a nil entry.
func ownedEnds(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	if u, ok := sym.Decl.(*ast.Usage); ok {
		for _, end := range u.ConnectorEnds {
			if end == nil {
				continue
			}
			if _, declares := end.DeclaredName(); !declares {
				out = append(out, nil)
				continue
			}
			out = append(out, memberSymbol(sym.Scope, end))
		}
	}
	for _, member := range declMembers(sym) {
		usage, ok := unwrapUsage(member)
		if !ok || !usage.IsEnd {
			continue
		}
		// An end with no registered symbol still occupies its position.
		out = append(out, memberSymbol(sym.Scope, usage))
	}
	return out
}

// endsOf returns the effective ends of the connector sym: its owned ends, in
// declaration order, followed by the ends it inherits and does not redefine.
// As with parameters, only a single general connector may leave ends
// inherited. The result is memoized.
func (m *Model) endsOf(sym *symbols.Symbol) []*symbols.Symbol {
	if cached, ok := m.ends[sym]; ok {
		return cached
	}
	// Guard against re-entrancy on cyclic specialization graphs.
	m.ends[sym] = nil

	out := ownedEnds(sym)

	var generals []*symbols.Symbol
	for _, sup := range m.DirectSupertypes(sym) {
		if connectorLike(sup) {
			generals = append(generals, sup)
		}
	}
	if len(generals) == 1 {
		general := m.endsOf(generals[0])
		claimed := claimedEnds(out, general)
		for _, end := range general {
			if !claimed[end] {
				out = append(out, end)
			}
		}
	}

	m.ends[sym] = out
	return out
}

// claimedEnds returns the ends of general that owned redefines, each by the
// target its declaration names explicitly or, failing that, the one at its own
// position. Only what no owned end claims is inherited.
func claimedEnds(owned, general []*symbols.Symbol) map[*symbols.Symbol]bool {
	claimed := make(map[*symbols.Symbol]bool)
	for i, end := range owned {
		if explicit := namedEnds(end, general); len(explicit) > 0 {
			for _, target := range explicit {
				claimed[target] = true
			}
			continue
		}
		if i < len(general) {
			claimed[general[i]] = true
		}
	}
	delete(claimed, nil)
	return claimed
}

// namedEnds returns the ends of general that end's `:>>` clauses name, matching
// on the last segment of each qualified name.
func namedEnds(end *symbols.Symbol, general []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	if end == nil {
		return nil
	}
	for _, rel := range RelationshipsOf(end) {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		name := qn.Parts[len(qn.Parts)-1].Text
		for _, candidate := range general {
			if candidate != nil && candidate.Name == name {
				out = append(out, candidate)
			}
		}
	}
	return out
}

// implicitEndRedefinitions returns the features sym implicitly redefines as an
// end of its owning connector: the end at the same position of each general
// connector. It returns nothing for a feature that is not an end, or whose
// declaration redefines something explicitly.
func (m *Model) implicitEndRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if !declaresEnd(sym) {
		return nil
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return nil // explicit redefinition governs
		}
	}
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !connectorLike(owner) {
		return nil
	}
	position := -1
	for i, end := range ownedEnds(owner) {
		if end == sym {
			position = i
			break
		}
	}
	if position < 0 {
		return nil
	}

	var out []*symbols.Symbol
	for _, sup := range m.DirectSupertypes(owner) {
		if !connectorLike(sup) {
			continue
		}
		supEnds := m.endsOf(sup)
		if position >= len(supEnds) {
			continue
		}
		if target := supEnds[position]; target != nil && target != sym {
			out = append(out, target)
		}
	}
	return out
}

// declaresEnd reports whether sym declares an end feature: an end of a
// `connect` clause, or a body feature declared with the `end` modifier.
func declaresEnd(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.ConnectorEnd:
		_, declares := d.DeclaredName()
		return declares
	case *ast.Usage:
		return d.IsEnd
	}
	return false
}

// UnmatchedConnectorEnds returns the ends the connector sym declares that
// redefine no end of a general connector, together with the general connector
// that has too few ends. A connector with no connector-like general — an
// untyped `connect a to b` — has none: there is nothing to match against.
func (m *Model) UnmatchedConnectorEnds(sym *symbols.Symbol) (*symbols.Symbol, []*symbols.Symbol) {
	if !connectorLike(sym) {
		return nil, nil
	}
	owned := ownedEnds(sym)
	if len(owned) == 0 {
		return nil, nil
	}
	for _, sup := range m.DirectSupertypes(sym) {
		if !connectorLike(sup) {
			continue
		}
		supEnds := m.endsOf(sup)
		if len(supEnds) == 0 {
			// The general's own ends are not enumerable (no parsed body, or they
			// are inherited from an unparsed library), so its arity is unknown.
			continue
		}
		var unmatched []*symbols.Symbol
		for i, end := range owned {
			if end == nil || i < len(supEnds) || len(namedEnds(end, supEnds)) > 0 {
				continue
			}
			unmatched = append(unmatched, end)
		}
		if len(unmatched) > 0 {
			return sup, unmatched
		}
	}
	return nil, nil
}

// ConnectorEndCount returns the number of effective ends of the connector sym.
func (m *Model) ConnectorEndCount(sym *symbols.Symbol) int {
	return len(m.endsOf(sym))
}
