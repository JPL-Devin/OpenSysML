package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

func (m *Model) isConnectorLike(sym *symbols.Symbol) bool {
	return connectorLike(sym) || sym != nil && sym.Decl == nil && m.IsBinaryConnector(sym)
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

	if sym != nil && sym.Decl == nil && m.IsBinaryConnector(sym) {
		var out []*symbols.Symbol
		for _, name := range binaryConnectorEndNames {
			end, ok := m.LookupMember(sym, name)
			if !ok {
				m.ends[sym] = nil
				return nil
			}
			out = append(out, end)
		}
		m.ends[sym] = out
		return out
	}

	out := ownedEnds(sym)

	if general := m.generalConnectorEnds(sym); len(general) > 0 {
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

// ImplicitEndRedefinitions returns the ends of the connectors its owner
// specializes that sym redefines by occupying their position, which no clause of
// its declaration names. The features it redefines are one feature with it, so
// an object holds one set of values under all their names.
func (m *Model) ImplicitEndRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	return m.implicitEndRedefinitions(sym)
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
	if !m.isConnectorLike(owner) {
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
		if !m.isConnectorLike(sup) {
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
	if sym == nil {
		return false
	}
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
	if !m.isConnectorLike(sym) {
		return nil, nil
	}
	owned := ownedEnds(sym)
	if len(owned) == 0 {
		return nil, nil
	}
	for _, sup := range m.DirectSupertypes(sym) {
		if !m.isConnectorLike(sup) {
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
	count := len(m.endsOf(sym))
	if count == 0 && sym != nil && sym.Decl == nil &&
		m.fqnOf(sym) == binaryConnectorBaseFQN {
		return len(binaryConnectorEndNames)
	}
	return count
}

// IsConnectorUsage reports whether sym declares a connector usage that joins
// features it names — a `connection`/`interface`/`allocation`/`connector` usage
// with a `connect … to …` clause. Such a usage is materialized from the
// features its ends attach to, unlike a usage that only holds objects of its
// own.
func (m *Model) IsConnectorUsage(sym *symbols.Symbol) bool {
	if sym == nil || !m.isConnectorLike(sym) {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	switch usage.Kind {
	case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation, ast.UsageConnector:
		return len(usage.ConnectorEnds) > 0
	}
	return false
}

// ConnectorEndAttachment is one end of a connector usage as an object of that
// usage carries it: the name of the end feature the position occupies, and the
// node naming the feature the end attaches to. A connector end
// reference-subsets what it attaches to (KerML 1.0 §7.4.6, SysML v2 §7.13.2),
// so an object of the connector holds that very feature at the end rather than
// an object of the end's declared type.
type ConnectorEndAttachment struct {
	// Name is the effective name of the end feature this position occupies: the
	// name the end declares for itself, else the name of the end it implicitly
	// redefines in the connector it specializes, else the name a binary
	// connector's ends have in the library. It is empty when none of those
	// answers a name — an end of an untyped connector with an arity other than
	// two, whose ends are the participants of a link and are unnamed.
	Name string
	// Attachment is the expression naming the connected feature (`a.p`).
	Attachment ast.Node
	// End is the syntax of the end itself, which carries its source location.
	End *ast.ConnectorEnd
	// EndFeature is the end feature Name comes from, when a declaration in the
	// model declares it: the end's own symbol, or the end it redefines. It is nil
	// for a name taken from the library, whose declarations are indexed without
	// bodies.
	EndFeature *symbols.Symbol
}

// binaryConnectorEndNames are the ends of every binary connector: a connector
// usage with two ends specializes `Connections::BinaryConnection` (SysML v2
// §8.3.13), whose `source` and `target` redefine those of `Links::BinaryLink`,
// and each of the usage's ends implicitly redefines the one at its position. A
// binary `interface`, `allocation` or `flow` reaches the same pair through
// `BinaryInterface`, `Allocation` and `Flow`, which redefine them again.
// An indexed base retains these two effective ends even without its body.
var binaryConnectorEndNames = [2]string{"source", "target"}

const binaryConnectorBaseFQN = "Links::BinaryLink"

// IsBinaryConnector reports whether sym conforms to the library's binary-link
// base, including through cached/index-only specialization edges.
func (m *Model) IsBinaryConnector(sym *symbols.Symbol) bool {
	return m != nil && m.conformsByName(sym, binaryConnectorBaseFQN)
}

// ConnectorEndAttachments returns the ends of the connector usage sym in
// declaration order, one entry per end of its `connect` clause. It returns
// nothing for a symbol that is no connector usage.
func (m *Model) ConnectorEndAttachments(sym *symbols.Symbol) []ConnectorEndAttachment {
	if !m.IsConnectorUsage(sym) {
		return nil
	}
	usage := sym.Decl.(*ast.Usage)
	owned := ownedEnds(sym)
	general := m.generalConnectorEnds(sym)

	out := make([]ConnectorEndAttachment, 0, len(usage.ConnectorEnds))
	for i, end := range usage.ConnectorEnds {
		if end == nil {
			continue
		}
		att := ConnectorEndAttachment{Attachment: end.AttachedTarget(), End: end}
		switch {
		case i < len(owned) && owned[i] != nil:
			att.Name, att.EndFeature = owned[i].Name, owned[i]
		case i < len(general) && general[i] != nil:
			att.Name, att.EndFeature = general[i].Name, general[i]
		case len(usage.ConnectorEnds) == 2:
			att.Name = binaryConnectorEndNames[i]
		}
		out = append(out, att)
	}
	return out
}

// FlowEndAttachment is one declared from/to target of a flow usage.
type FlowEndAttachment struct {
	Attachment ast.Node
}

// FlowEndAttachments returns the declared from/to targets of a flow usage.
func (m *Model) FlowEndAttachments(sym *symbols.Symbol) []FlowEndAttachment {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageFlow || usage.Keyword == "message" || usage.FlowEnds == nil {
		return nil
	}
	out := make([]FlowEndAttachment, 0, 2)
	for _, target := range []ast.Node{usage.FlowEnds.From, usage.FlowEnds.To} {
		if target == nil {
			continue
		}
		out = append(out, FlowEndAttachment{Attachment: target})
	}
	return out
}

// generalConnectorEnds returns the effective ends of the connector sym
// specializes, which its own ends redefine by position. As with parameters,
// only a single general connector may supply them, and a general whose ends are
// not enumerable — a library declaration indexed without its body — supplies
// none.
func (m *Model) generalConnectorEnds(sym *symbols.Symbol) []*symbols.Symbol {
	var generals [][]*symbols.Symbol
	for _, sup := range m.DirectSupertypes(sym) {
		if !m.isConnectorLike(sup) {
			continue
		}
		if ends := m.endsOf(sup); len(ends) > 0 {
			generals = append(generals, ends)
		}
	}
	if len(generals) != 1 {
		return nil
	}
	return generals[0]
}
