// Package semantics provides the derived semantic model that validation
// depth-C constraint checks rely on: a specialization/typing graph (with cycle
// detection), and — in later increments — inherited-member resolution,
// multiplicity extraction, and a bounded model-level expression evaluator.
//
// All results are memoized in side tables keyed by *symbols.Symbol, consistent
// with the project rule that semantic information lives outside the immutable
// AST. A Model is built per resolution session over an existing symbol index
// and name resolver.
package semantics

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Model is the derived semantic model over a symbol index. It memoizes the
// specialization graph computed from resolved def/usage relationships.
type Model struct {
	resolver *resolve.Resolver

	directSupers map[*symbols.Symbol][]*symbols.Symbol
	allSupers    map[*symbols.Symbol][]*symbols.Symbol
}

// NewModel creates a semantic model backed by the given name resolver. The
// resolver must already be associated with the index whose symbols will be
// queried.
func NewModel(resolver *resolve.Resolver) *Model {
	return &Model{
		resolver:     resolver,
		directSupers: make(map[*symbols.Symbol][]*symbols.Symbol),
		allSupers:    make(map[*symbols.Symbol][]*symbols.Symbol),
	}
}

// GeneralizationKind reports whether a relationship kind forms a conformance
// ("is-a" / "conforms-to") edge for the specialization graph: specialization on
// definitions, and subsetting/redefinition/typing on usages. references/crosses
// are feature-value edges, not generalization, and are excluded.
func GeneralizationKind(k ast.RelationshipKind) bool {
	switch k {
	case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelTyping:
		return true
	default:
		return false
	}
}

// RelationshipsOf returns the declared relationships of a symbol's def/usage
// declaration, or nil for symbols that are not def/usage.
func RelationshipsOf(sym *symbols.Symbol) []*ast.Relationship {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Relationships
	case *ast.Usage:
		return d.Relationships
	default:
		return nil
	}
}

// DirectSupertypes returns the immediate supertype symbols of sym: the resolved
// targets of its generalization relationships. Unresolved or non-def/usage
// targets are skipped. The result is memoized and deterministic (declaration
// order, duplicates removed).
func (m *Model) DirectSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.directSupers[sym]; ok {
		return cached
	}
	// Guard against re-entrancy on cyclic graphs: seed with an empty slice.
	m.directSupers[sym] = nil

	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		// Unwrap FeatureReference if needed
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, isQN := targetNode.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		target, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || target == nil {
			continue
		}
		// Skip self-reference ONLY for redefines (nested feature case)
		// Preserve self-reference for specializes/typing to detect cycles
		if target == sym && rel.Kind == ast.RelRedefines {
			continue
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	
	// SubjectMember has TypeRef instead of Relationships - handle separately
	if subj, ok := sym.Decl.(*ast.SubjectMember); ok && subj.TypeRef != nil {
		if target, ok := m.resolver.ResolveQualified(sym.OwnerScope, subj.TypeRef); ok && target != nil {
			if !seen[target] {
				seen[target] = true
				out = append(out, target)
			}
		}
	}
	
	m.directSupers[sym] = out
	return out
}

// AllSupertypes returns the transitive closure of DirectSupertypes, excluding
// sym itself, in a deterministic order (breadth-first over declaration order).
// It is safe on cyclic graphs. The result is memoized.
func (m *Model) AllSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.allSupers[sym]; ok {
		return cached
	}

	var order []*symbols.Symbol
	visited := make(map[*symbols.Symbol]bool)
	queue := append([]*symbols.Symbol(nil), m.DirectSupertypes(sym)...)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == sym || visited[cur] {
			continue
		}
		visited[cur] = true
		order = append(order, cur)
		queue = append(queue, m.DirectSupertypes(cur)...)
	}
	m.allSupers[sym] = order
	return order
}

// Conforms reports whether a conforms to b: a == b, or b is a (transitive)
// supertype of a.
func (m *Model) Conforms(a, b *symbols.Symbol) bool {
	if a == nil || b == nil {
		return false
	}
	if a == b {
		return true
	}
	for _, s := range m.AllSupertypes(a) {
		if s == b {
			return true
		}
	}
	return false
}

// HasSpecializationCycle reports whether sym participates in a specialization
// cycle: sym is reachable from itself through one or more generalization edges
// (including a direct self-specialization). AllSupertypes excludes its own
// starting node, so sym is detected via a back-edge from one of its supertypes.
func (m *Model) HasSpecializationCycle(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	for _, s := range m.DirectSupertypes(sym) {
		if s == sym {
			return true // direct self-specialization
		}
		for _, up := range m.AllSupertypes(s) {
			if up == sym {
				return true
			}
		}
	}
	return false
}
