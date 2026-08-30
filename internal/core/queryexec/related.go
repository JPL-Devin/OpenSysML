package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Relationship kinds RelatedElements traverses. A lineage kind follows the
// declared relationships of the element itself; the others follow edges other
// declarations state about it (a connector usage, a satisfy/verify assertion).
const (
	relationshipSpecialization = "specialization"
	relationshipSubsetting     = "subsetting"
	relationshipRedefinition   = "redefinition"
	relationshipTyping         = "typing"
	relationshipConnection     = "connection"
	relationshipAllocation     = "allocation"
	relationshipSatisfaction   = "satisfaction"
	relationshipVerification   = "verification"
)

// Traversal directions: outgoing follows an edge from its source to its
// targets, incoming follows it in reverse.
const (
	directionOutgoing = "outgoing"
	directionIncoming = "incoming"
)

// lineageKinds maps the lineage relationship kinds to the AST relationship
// they follow.
var lineageKinds = map[string]ast.RelationshipKind{
	relationshipSpecialization: ast.RelSpecializes,
	relationshipSubsetting:     ast.RelSubsets,
	relationshipRedefinition:   ast.RelRedefines,
	relationshipTyping:         ast.RelTyping,
}

// relationshipEdges holds the resolved edges of one relationship kind, in
// document order then declaration order, keyed by semantic identity.
type relationshipEdges struct {
	outgoing map[symbols.ElementKey][]*symbols.Symbol
	incoming map[symbols.ElementKey][]*symbols.Symbol
}

// relationshipTables caches the per-kind edge tables of one execution, shared
// across invoked queries the way the visit budget is.
type relationshipTables struct {
	entries map[string]*relationshipEdges
}

func newRelationshipTables() *relationshipTables {
	return &relationshipTables{entries: make(map[string]*relationshipEdges)}
}

func (e *executor) evaluateRelated(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	kind, err := e.stringArgument(expression, "relationshipKind")
	if err != nil {
		return sequence{}, err
	}
	direction, err := e.stringArgument(expression, "direction")
	if err != nil {
		return sequence{}, err
	}
	maxDepth, err := e.integerArgument(expression, "maxDepth")
	if err != nil {
		return sequence{}, err
	}
	if !supportedRelationship(kind) {
		return sequence{}, &Error{
			Kind:      ErrorUnknownRelationship,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    kind,
			Origin:    expression.Origin(),
		}
	}
	if direction != directionOutgoing && direction != directionIncoming {
		return sequence{}, e.operatorError(expression, direction)
	}
	type pending struct {
		sym   *symbols.Symbol
		depth int64
	}
	queue := make([]pending, 0, len(source.values))
	seen := make(map[symbols.ElementKey]struct{})
	for _, value := range source.values {
		sym, _ := value.Element()
		seen[symbols.KeyOf(sym)] = struct{}{}
		queue = append(queue, pending{sym: sym})
	}
	var result sequence
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next.depth >= maxDepth {
			continue
		}
		for _, neighbor := range e.relatedNeighbors(kind, direction, next.sym) {
			key := symbols.KeyOf(neighbor)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			seen[key] = struct{}{}
			result.values = append(result.values, ElementValue(neighbor))
			queue = append(queue, pending{sym: neighbor, depth: next.depth + 1})
		}
	}
	return result, nil
}

func supportedRelationship(kind string) bool {
	if _, ok := lineageKinds[kind]; ok {
		return true
	}
	switch kind {
	case relationshipConnection, relationshipAllocation,
		relationshipSatisfaction, relationshipVerification:
		return true
	}
	return false
}

// relatedNeighbors returns the elements one edge of the given kind away from
// sym in the given direction, in declaration order. Outgoing lineage reads
// sym's own declared relationships; every other combination reads the edge
// tables built from the workspace's declarations.
func (e *executor) relatedNeighbors(kind, direction string, sym *symbols.Symbol) []*symbols.Symbol {
	if relKind, lineage := lineageKinds[kind]; lineage && direction == directionOutgoing {
		return e.lineageTargets(sym, relKind)
	}
	edges := e.relationshipEdges(kind)
	table := edges.outgoing
	if direction == directionIncoming {
		table = edges.incoming
	}
	return table[symbols.KeyOf(sym)]
}

// lineageTargets resolves the targets of sym's declared relationships of the
// given kind, in declaration order.
func (e *executor) lineageTargets(sym *symbols.Symbol, kind ast.RelationshipKind) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != kind || rel.Target == nil {
			continue
		}
		if target, ok := e.context.Resolver.ResolveTarget(sym.OwnerScope, rel.Target); ok && target != nil {
			out = append(out, target)
		}
	}
	return out
}

// relationshipEdges returns the edge tables for one relationship kind,
// building them on first use by scanning the workspace's documents in sorted
// name order and each document's symbols in declaration order.
func (e *executor) relationshipEdges(kind string) *relationshipEdges {
	if cached, ok := e.related.entries[kind]; ok {
		return cached
	}
	edges := &relationshipEdges{
		outgoing: make(map[symbols.ElementKey][]*symbols.Symbol),
		incoming: make(map[symbols.ElementKey][]*symbols.Symbol),
	}
	for _, document := range e.context.Index.WorkspaceDocuments() {
		e.scanScope(edges, kind, e.context.Index.DocumentRoot(document))
	}
	e.related.entries[kind] = edges
	return edges
}

// scanScope records the edges of one relationship kind that the declarations
// in scope and its nested scopes state.
func (e *executor) scanScope(edges *relationshipEdges, kind string, scope *symbols.Scope) {
	if scope == nil {
		return
	}
	for _, member := range scope.AllMembers() {
		e.scanSymbol(edges, kind, member)
	}
	for _, child := range scope.Children() {
		e.scanScope(edges, kind, child)
	}
}

// scanSymbol records the edges the given symbol's declaration states: the
// resolved targets of a lineage relationship, the resolved end features of a
// connector usage, or the subject and requirement of a satisfaction assertion.
func (e *executor) scanSymbol(edges *relationshipEdges, kind string, sym *symbols.Symbol) {
	if relKind, lineage := lineageKinds[kind]; lineage {
		for _, target := range e.lineageTargets(sym, relKind) {
			addEdge(edges, sym, target)
		}
		return
	}
	switch kind {
	case relationshipConnection, relationshipAllocation:
		e.scanConnector(edges, kind, sym)
	case relationshipSatisfaction, relationshipVerification:
		e.scanSatisfaction(edges, kind, sym)
	}
}

// scanConnector records the edges a connector usage states: from the feature
// its first end attaches to, to the feature of each later end, in declaration
// order.
func (e *executor) scanConnector(edges *relationshipEdges, kind string, sym *symbols.Symbol) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !connectorRelationship(usage.Kind, kind) || !e.context.Model.IsConnectorUsage(sym) {
		return
	}
	var ends []*symbols.Symbol
	for _, attachment := range e.context.Model.ConnectorEndAttachments(sym) {
		if attachment.Attachment == nil {
			continue
		}
		target, ok := e.context.Resolver.ResolveTarget(sym.OwnerScope, attachment.Attachment)
		if !ok || target == nil {
			continue
		}
		ends = append(ends, target)
	}
	if len(ends) < 2 {
		return
	}
	for _, target := range ends[1:] {
		addEdge(edges, ends[0], target)
	}
}

// connectorRelationship reports whether a connector usage of the given AST
// kind carries edges of the named relationship kind. Connection covers the
// connection, connector, and interface usages; allocation stands alone.
func connectorRelationship(usage ast.UsageKind, kind string) bool {
	switch kind {
	case relationshipConnection:
		return usage == ast.UsageConnection || usage == ast.UsageConnector || usage == ast.UsageInterface
	case relationshipAllocation:
		return usage == ast.UsageAllocation
	}
	return false
}

// scanSatisfaction records the edge a satisfy or verify assertion states: from
// the subject its `by` clause names — else the element stating the assertion —
// to the requirement it references.
func (e *executor) scanSatisfaction(edges *relationshipEdges, kind string, sym *symbols.Symbol) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageSatisfy {
		return
	}
	if (usage.Keyword == "verify") != (kind == relationshipVerification) {
		return
	}
	var requirement, subject *symbols.Symbol
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		target, ok := e.context.Resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets:
			requirement = target
		case ast.RelSubject:
			subject = target
		}
	}
	if subject == nil && sym.OwnerScope != nil {
		subject = sym.OwnerScope.Owner()
	}
	if subject == nil || requirement == nil {
		return
	}
	addEdge(edges, subject, requirement)
}

// addEdge records one source-to-target edge in both directions.
func addEdge(edges *relationshipEdges, source, target *symbols.Symbol) {
	sourceKey, targetKey := symbols.KeyOf(source), symbols.KeyOf(target)
	edges.outgoing[sourceKey] = append(edges.outgoing[sourceKey], target)
	edges.incoming[targetKey] = append(edges.incoming[targetKey], source)
}
