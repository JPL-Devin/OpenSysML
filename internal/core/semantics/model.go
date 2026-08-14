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

	directSupers  map[*symbols.Symbol][]*symbols.Symbol
	allSupers     map[*symbols.Symbol][]*symbols.Symbol
	referenced    map[*symbols.Symbol]*symbols.Symbol
	resolvingRef  map[*symbols.Symbol]bool
	memberSources map[*symbols.Symbol][]*symbols.Symbol
	primTypes     map[*symbols.Symbol]PrimType
	scalars       map[*symbols.Symbol]PrimType // stdlib scalar symbols, resolved once
	params        map[*symbols.Symbol]behaviorParameters
	ends          map[*symbols.Symbol][]*symbols.Symbol

	superEdgeCache map[*symbols.Symbol][]superEdge      // generalization edges with conjugation
	conjSupers     map[*symbols.Symbol][]conjugatedType // supertypes with conjugation parity

	unitTerms    map[*symbols.Symbol]UnitTerm // measurement units reduced to base units
	reducingUnit map[*symbols.Symbol]bool     // units being reduced, to detect a cycle
	libSymbols   map[string]*symbols.Symbol   // library elements resolved by qualified name
}

// NewModel creates a semantic model backed by the given name resolver. The
// resolver must already be associated with the index whose symbols will be
// queried. The model attaches itself to the resolver so name resolution sees
// inherited members, which a redefinition target may only be reachable through.
func NewModel(resolver *resolve.Resolver) *Model {
	m := &Model{
		resolver:      resolver,
		directSupers:  make(map[*symbols.Symbol][]*symbols.Symbol),
		allSupers:     make(map[*symbols.Symbol][]*symbols.Symbol),
		referenced:    make(map[*symbols.Symbol]*symbols.Symbol),
		resolvingRef:  make(map[*symbols.Symbol]bool),
		memberSources: make(map[*symbols.Symbol][]*symbols.Symbol),
		primTypes:     make(map[*symbols.Symbol]PrimType),
		params:        make(map[*symbols.Symbol]behaviorParameters),
		ends:          make(map[*symbols.Symbol][]*symbols.Symbol),

		superEdgeCache: make(map[*symbols.Symbol][]superEdge),
		conjSupers:     make(map[*symbols.Symbol][]conjugatedType),
		unitTerms:      make(map[*symbols.Symbol]UnitTerm),
		reducingUnit:   make(map[*symbols.Symbol]bool),
		libSymbols:     make(map[string]*symbols.Symbol),
	}
	if resolver != nil {
		resolver.SetModel(m)
	}
	return m
}

// GeneralizationKind reports whether a relationship kind forms a conformance
// ("is-a" / "conforms-to") edge for the specialization graph: specialization on
// definitions, and subsetting/redefinition/typing on usages.
//
// Reference subsetting (`references`) is excluded even though KerML 8.3.3.3.9
// makes it a kind of Subsetting: it contributes members through MemberSources
// instead, so that a referencing feature does not silently acquire the
// referenced feature's type for conformance and implicit-typing purposes.
// crosses is a feature-value edge, not generalization, and is excluded too.
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
	case *ast.ConnectorEnd:
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
		// A redefinition names the feature it refines, which the redefining
		// feature shadows in its own scope (`part redefines engine`), so a
		// target resolving to sym itself must be looked up in what sym's owner
		// inherits. Self-reference through specializes/typing is preserved so
		// cycle detection still sees it.
		if target == sym && rel.Kind == ast.RelRedefines {
			if redefined := m.inheritedFeature(sym, qn); redefined != nil {
				target = redefined
			} else {
				continue
			}
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}

	// A library symbol restored from the cache has no AST: its specialization
	// edges were resolved when the record was written and are carried as FQNs.
	for _, fqn := range sym.SuperFQNs {
		for _, target := range m.resolver.Index().LookupQualified(fqn) {
			if target == nil || target == sym || seen[target] {
				continue
			}
			seen[target] = true
			out = append(out, target)
			break
		}
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

	// Flow usages (message/flow keywords) need implicit typing from stdlib Message/Flow
	// even when explicitly typed "of Type". This allows accessing stdlib members like
	// sourceEvent/targetEvent in messages typed by payload item defs.
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageFlow {
		// Look up stdlib Message flow def directly from index (FQN: Flows::Message)
		messageDefs := m.resolver.Index().LookupQualified("Flows::Message")
		if len(messageDefs) > 0 && messageDefs[0] != nil {
			messageDef := messageDefs[0]
			if !seen[messageDef] {
				seen[messageDef] = true
				out = append(out, messageDef)
			}
		}
	}

	// Action usages containing send statements need implicit SendAction typing
	// to provide access to sentMessage member
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
		if hasSendStatement(usage) {
			sendActionDefs := m.resolver.Index().LookupQualified("Actions::SendAction")
			if len(sendActionDefs) > 0 && sendActionDefs[0] != nil {
				sendActionDef := sendActionDefs[0]
				if !seen[sendActionDef] {
					seen[sendActionDef] = true
					out = append(out, sendActionDef)
				}
			}
		}
	}

	// A variant specializes the variation it is a variant of, so it carries the
	// variation's type and features and restates only what it chooses
	// (SysML v2 §7.20).
	if variation := VariationOwning(sym); variation != nil && variation != sym && !seen[variation] {
		seen[variation] = true
		out = append(out, variation)
	}

	// Semantic metadata annotating this element — a `#keyword` prefix — adds the
	// implicit specialization of its baseType (SysML v2 §7.27.3, §7.27.4).
	for _, base := range m.semanticMetadataBases(sym) {
		if base == nil || base == sym || seen[base] {
			continue
		}
		seen[base] = true
		out = append(out, base)
	}

	// A parameter of a behavior or step implicitly redefines the corresponding
	// parameter of each behavior or step its owner specializes, and so takes
	// that parameter's type when it declares none (see redefinition.go).
	declared := len(out)
	for _, redefined := range m.implicitParameterRedefinitions(sym) {
		if seen[redefined] {
			continue
		}
		seen[redefined] = true
		out = append(out, redefined)
	}

	// An end of a connector implicitly redefines the end at its own position of
	// each connector its owner specializes, and so takes that end's type when it
	// declares none (see connector.go).
	for _, redefined := range m.implicitEndRedefinitions(sym) {
		if seen[redefined] {
			continue
		}
		seen[redefined] = true
		out = append(out, redefined)
	}

	// An untyped usage still specializes its standard-library base feature.
	// Implicit redefinition does not stand in for it: the two rules are
	// independent, and the redefined parameter may itself be untyped.
	if declared == 0 {
		if base := m.implicitBase(sym); base != nil {
			out = append(out, base)
		}
	}

	m.directSupers[sym] = out
	return out
}

// inheritedFeature returns the feature that sym's owner inherits under the name
// qn denotes, skipping the owner's own members so a redefinition does not find
// itself. Only a single-segment name can denote an inherited feature this way;
// a qualified one names its owner explicitly.
func (m *Model) inheritedFeature(sym *symbols.Symbol, qn *ast.QualifiedName) *symbols.Symbol {
	if len(qn.Parts) != 1 {
		return nil
	}
	return m.inheritedFeatureNamed(sym, qn.Parts[0].Text)
}

// inheritedFeatureNamed is inheritedFeature for an already-extracted name.
func (m *Model) inheritedFeatureNamed(sym *symbols.Symbol, name string) *symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return nil
	}
	for _, sup := range m.AllSupertypes(owner) {
		if found, ok := m.LookupMember(sup, name); ok && found != sym {
			return found
		}
	}
	return nil
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
	if a == b || isAnything(b) {
		return true
	}
	for _, s := range m.AllSupertypes(a) {
		if s == b {
			return true
		}
	}
	return false
}

// isAnything reports whether sym is Base::Anything, the classifier every type
// specializes (KerML 8.3.2.1), whether or not the chain to it is declared.
func isAnything(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Name == "Base::Anything" ||
		(sym.Name == "Anything" && sym.OwnerScope != nil && sym.OwnerScope.Owner() != nil &&
			sym.OwnerScope.Owner().Name == "Base"))
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

// hasSendStatement checks if an action usage contains a send statement in its body
func hasSendStatement(usage *ast.Usage) bool {
	for _, member := range usage.Members {
		if _, ok := member.(*ast.SendStatement); ok {
			return true
		}
	}
	return false
}
