package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Port conjugation (SysML v2 §7.12.2, §7.12.3): `port p : ~P` is a usage of the
// conjugated definition of P, which has P's features with in/out reversed.

// PortFeature is a feature of a port, with the direction it has as seen through
// the port that was queried.
type PortFeature struct {
	Symbol    *symbols.Symbol
	Name      string
	Direction ast.FeatureDirection
}

// conjugatedType is a type a port takes features from, with the parity of the
// conjugations reaching it.
type conjugatedType struct {
	sym        *symbols.Symbol
	conjugated bool
}

// superEdge is one generalization edge, with whether it conjugates its target
// and whether it is the feature typing that states the declaration's type.
type superEdge struct {
	sym        *symbols.Symbol
	conjugated bool
	typing     bool
}

// ConjugateDirection returns the conjugate of a feature direction (§7.12.2):
// in and out are each other's conjugate, inout and none are their own.
func ConjugateDirection(d ast.FeatureDirection) ast.FeatureDirection {
	switch d {
	case ast.DirIn:
		return ast.DirOut
	case ast.DirOut:
		return ast.DirIn
	default:
		return d
	}
}

// IsConjugated reports whether sym's inherited features have reversed
// directions. Conjugation composes, so `~` of a conjugate is the original.
func (m *Model) IsConjugated(sym *symbols.Symbol) bool {
	parity := false
	visited := make(map[*symbols.Symbol]bool)
	for cur := sym; cur != nil && !visited[cur]; {
		visited[cur] = true
		edge, ok := m.typeEdge(cur)
		if !ok {
			break
		}
		parity = parity != edge.conjugated
		cur = edge.sym
	}
	return parity
}

// typeEdge returns the edge that gives sym its port type: the feature typing
// (`~` types a port, so a redefinition declared before it must not hide it),
// else the first generalization.
func (m *Model) typeEdge(sym *symbols.Symbol) (superEdge, bool) {
	edges := m.superEdges(sym)
	for _, edge := range edges {
		if edge.typing {
			return edge, true
		}
	}
	if len(edges) == 0 {
		return superEdge{}, false
	}
	return edges[0], true
}

// featureEdges returns sym's generalization edges with the one giving it its
// type first, so a feature of that type masks a same-named inherited one.
func (m *Model) featureEdges(sym *symbols.Symbol) []superEdge {
	edges := m.superEdges(sym)
	typed, ok := m.typeEdge(sym)
	if !ok || !typed.typing {
		return edges
	}
	out := make([]superEdge, 0, len(edges))
	out = append(out, typed)
	for _, edge := range edges {
		if edge.sym != typed.sym {
			out = append(out, edge)
		}
	}
	return out
}

// superEdges returns sym's generalization edges in declaration order, each with
// whether the relationship conjugates its target.
func (m *Model) superEdges(sym *symbols.Symbol) []superEdge {
	if sym == nil {
		return nil
	}
	if cached, ok := m.superEdgeCache[sym]; ok {
		return cached
	}
	m.superEdgeCache[sym] = nil

	var out []superEdge
	seen := make(map[*symbols.Symbol]bool)
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, isQN := target.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		resolved, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || resolved == nil || resolved == sym || seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, superEdge{sym: resolved, conjugated: rel.Conjugated, typing: rel.Kind == ast.RelTyping})
	}
	// Supertypes known beyond declared relationships never conjugate.
	for _, sup := range m.DirectSupertypes(sym) {
		if !seen[sup] {
			seen[sup] = true
			out = append(out, superEdge{sym: sup})
		}
	}
	m.superEdgeCache[sym] = out
	return out
}

// conjugatedSupertypes returns the types sym takes features from — sym first,
// then its supertypes breadth-first — with the conjugation parity of each.
func (m *Model) conjugatedSupertypes(sym *symbols.Symbol) []conjugatedType {
	if sym == nil {
		return nil
	}
	if cached, ok := m.conjSupers[sym]; ok {
		return cached
	}
	m.conjSupers[sym] = nil

	out := []conjugatedType{{sym: sym}}
	visited := map[*symbols.Symbol]bool{sym: true}
	for i := 0; i < len(out); i++ {
		cur := out[i]
		for _, edge := range m.featureEdges(cur.sym) {
			if visited[edge.sym] {
				continue
			}
			visited[edge.sym] = true
			out = append(out, conjugatedType{
				sym:        edge.sym,
				conjugated: edge.conjugated != cur.conjugated,
			})
		}
	}
	m.conjSupers[sym] = out
	return out
}

// PortFeatures returns the features of the port sym, declared and inherited,
// with the direction each has as seen through sym. A closer declaration masks
// an inherited feature of the same name.
func (m *Model) PortFeatures(sym *symbols.Symbol) []PortFeature {
	if sym == nil {
		return nil
	}
	var out []PortFeature
	seenName := make(map[string]bool)
	for _, typ := range m.conjugatedSupertypes(sym) {
		if typ.sym == nil || typ.sym.Scope == nil {
			continue
		}
		names := make(map[string]bool)
		for _, member := range declMembers(typ.sym) {
			usage, ok := unwrapUsage(member)
			if !ok {
				continue
			}
			memberSym := memberSymbol(typ.sym.Scope, usage)
			name := usage.Ident.Name
			if name != "" && seenName[name] {
				continue
			}
			dir := usage.Direction
			if typ.conjugated {
				dir = ConjugateDirection(dir)
			}
			out = append(out, PortFeature{Symbol: memberSym, Name: name, Direction: dir})
			if name != "" {
				names[name] = true
			}
		}
		for name := range names {
			seenName[name] = true
		}
	}
	return out
}

// PortsConform reports whether every feature of port a matches one on port b
// (§7.12.2): conforming types, and conjugate or absent directions.
func (m *Model) PortsConform(a, b *symbols.Symbol) bool {
	if a == nil || b == nil {
		return false
	}
	return m.featuresMatchConjugate(m.PortFeatures(a), m.PortFeatures(b))
}

// featuresMatchConjugate reports whether every named directed feature in
// features has a counterpart in others with a conforming type and the conjugate
// direction. Undirected features carry no flow, so they impose nothing.
func (m *Model) featuresMatchConjugate(features, others []PortFeature) bool {
	for _, feature := range features {
		if feature.Name == "" {
			continue // an unnamed feature has nothing to be matched by name
		}
		if feature.Direction == ast.DirNone {
			continue // conjugation constrains directed features only (§7.12.2)
		}
		match, ok := findPortFeature(others, feature.Name)
		if !ok {
			return false
		}
		if match.Direction != ConjugateDirection(feature.Direction) {
			return false
		}
		if !m.featureTypesConform(feature.Symbol, match.Symbol) {
			return false
		}
	}
	return true
}

// InterfaceEndPortMismatch returns the port types of the two ends of the
// interface sym when they do not match with conjugate directions (§7.12.2).
// Ends whose port type is not resolvable, and interfaces whose ends are not
// both declared, are not reported.
func (m *Model) InterfaceEndPortMismatch(sym *symbols.Symbol) (a, b *symbols.Symbol, mismatch bool) {
	if !interfaceLike(sym) {
		return nil, nil, false
	}
	ends := m.endsOf(sym)
	if len(ends) != 2 {
		return nil, nil, false
	}
	first, firstFeatures, ok := m.endPortFeatures(ends[0])
	if !ok {
		return nil, nil, false
	}
	second, secondFeatures, ok := m.endPortFeatures(ends[1])
	if !ok {
		return nil, nil, false
	}
	if m.featuresMatchConjugate(firstFeatures, secondFeatures) {
		return nil, nil, false
	}
	return first, second, true
}

// endPortFeatures returns the port definition typing the end feature sym and
// the features of that port as seen through the end, so a `~P` end reports
// reversed directions (§7.12.2).
func (m *Model) endPortFeatures(sym *symbols.Symbol) (*symbols.Symbol, []PortFeature, bool) {
	typ, conjugated := m.portTypeOf(sym)
	if typ == nil {
		return nil, nil, false
	}
	features := m.PortFeatures(typ)
	if !conjugated {
		return typ, features, true
	}
	out := make([]PortFeature, len(features))
	for i, feature := range features {
		feature.Direction = ConjugateDirection(feature.Direction)
		out[i] = feature
	}
	return typ, out, true
}

// interfaceLike reports whether sym declares an interface definition or usage.
func interfaceLike(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefInterface
	case *ast.Usage:
		return d.Kind == ast.UsageInterface
	}
	return false
}

// portTypeOf returns the port definition typing the feature sym and whether the
// typing conjugates it.
func (m *Model) portTypeOf(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if sym == nil {
		return nil, false
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, isQN := target.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		resolved, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || resolved == nil {
			continue
		}
		if d, isDef := resolved.Decl.(*ast.Definition); !isDef || d.Kind != ast.DefPort {
			return nil, false
		}
		return resolved, rel.Conjugated
	}
	return nil, false
}

// findPortFeature returns the feature named name, if any.
func findPortFeature(features []PortFeature, name string) (PortFeature, bool) {
	for _, feature := range features {
		if feature.Name == name {
			return feature, true
		}
	}
	return PortFeature{}, false
}

// featureTypesConform reports whether two matched features' types conform in
// either direction. An unresolved type conforms; it is reported on its own.
func (m *Model) featureTypesConform(a, b *symbols.Symbol) bool {
	typeA := m.featureType(a)
	typeB := m.featureType(b)
	if typeA == nil || typeB == nil {
		return true
	}
	return m.Conforms(typeA, typeB) || m.Conforms(typeB, typeA)
}

// featureType returns the symbol a feature's typing relationship names, or nil.
func (m *Model) featureType(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, isQN := target.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		if resolved, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn); ok {
			return resolved
		}
	}
	return nil
}
