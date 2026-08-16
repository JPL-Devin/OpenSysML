package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/quickfix"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/suggest"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// CodeNotAVertex marks an endpoint naming an element no transition can start or
// end at (SysML 7.19.2).
const CodeNotAVertex = "not-a-vertex"

// ResolveEndpoint resolves the vertex a transition endpoint names, reporting one
// that names nothing or no vertex. A nil name is the sourceless form: legal.
func (r *Resolver) ResolveEndpoint(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qn == nil || len(qn.Parts) == 0 {
		return nil, false
	}
	if res, done := r.endpoints[qn]; done {
		return res.sym, res.ok
	}
	sym, ok := r.lookupEndpoint(scope, qn)
	res := resolution{sym: sym, ok: ok}
	switch {
	case !ok:
		last := qn.Parts[len(qn.Parts)-1]
		r.report(Diagnostic{
			Span:    qn.Span(),
			Message: r.endpointMessage(scope, qn),
			Code:    "unresolved",
			// A suggestion names a vertex, so it replaces the last segment alone,
			// leaving the qualifiers saying which state or region it lives in.
			Fixes: endpointFixes(last.Text, last.Span, r.vertexSuggestions(scope, qn)),
		})
	case !isVertex(sym.Decl):
		r.report(Diagnostic{
			Span:    qn.Span(),
			Message: "transition endpoint " + qnText(qn) + " is not a state or pseudostate",
			Code:    CodeNotAVertex,
		})
		res = resolution{}
	}
	if res.ok || r.quiet == 0 {
		r.endpoints[qn] = res
	}
	return res.sym, res.ok
}

// Endpoint returns the declaration an endpoint names, which lowering builds its
// edges from (lower.Endpoints); the lookup itself reports nothing. A failure is
// reported only when this resolver already resolved the endpoint out loud, which
// is what a memoized failure records; lowering reports the rest.
func (r *Resolver) Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool, bool) {
	var sym *symbols.Symbol
	var ok bool
	r.aside(func() { sym, ok = r.ResolveEndpoint(scope, qn) })
	if ok && sym != nil {
		return sym.Decl, true, false
	}
	_, reported := r.endpoints[qn]
	return nil, false, reported
}

// VertexInScope finds the vertex an endpoint names from the scope tree alone,
// innermost scope first, for a machine lowered without a resolver over its
// document (lower.ToStateGraph): no index is consulted, so nothing outside the
// scopes the endpoint was written in can answer it.
func VertexInScope(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	if scope == nil || qn == nil || len(qn.Parts) == 0 {
		return nil, false
	}
	parts := qualifiedParts(qn)
	machine := machineScope(scope)
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := firstVertex(s, parts); ok {
			return sym.Decl, true
		}
		if s == machine {
			break
		}
	}
	return nil, false
}

// lookupEndpoint finds what an endpoint names: the declaration ordinary lookup
// reaches, or else a vertex of the enclosing machine, which a transition may
// name across a region or into a nested state (UML 2.5.1 14.2.3.9).
func (r *Resolver) lookupEndpoint(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	var sym *symbols.Symbol
	var ok bool
	r.aside(func() { sym, ok = r.resolveQualified(scope, qn, nil) })
	if ok && isVertex(sym.Decl) {
		return sym, true
	}
	if vertex, found := firstVertex(machineScope(scope), qualifiedParts(qn)); found {
		return vertex, true
	}
	return sym, ok
}

// endpointMessage is what an endpoint resolving to nothing reports: the ordinary
// unresolved wording, hinting the vertices of the machine it was written in.
func (r *Resolver) endpointMessage(scope *symbols.Scope, qn *ast.QualifiedName) string {
	name := qnText(qn)
	return suggest.With(unresolvedReferencePrefix+name, name, r.vertexSuggestions(scope, qn))
}

// vertexSuggestions ranks the vertex names of the enclosing machine near the
// endpoint's spelling: a misspelled endpoint means a vertex of that machine.
func (r *Resolver) vertexSuggestions(scope *symbols.Scope, qn *ast.QualifiedName) []string {
	parts := qualifiedParts(qn)
	return suggest.Nearest(parts[len(parts)-1], vertexNames(machineScope(scope)))
}

// endpointFixes offers each suggested vertex as an edit replacing the endpoint.
func endpointFixes(name string, span source.Span, cands []string) []quickfix.Fix {
	if span.Len == 0 {
		return nil
	}
	fixes := make([]quickfix.Fix, 0, len(cands))
	for _, cand := range cands {
		if cand == name {
			continue
		}
		fixes = append(fixes, quickfix.Fix{
			Title:     "Change '" + name + "' to '" + cand + "'",
			Edits:     []quickfix.Edit{quickfix.Replace(span, cand)},
			Preferred: len(cands) == 1,
		})
	}
	if len(fixes) == 0 {
		return nil
	}
	return fixes
}

// isVertex reports whether decl is a vertex a transition may name: a state, a
// pseudostate, or a control node standing in for one (SysML 7.19.2).
func isVertex(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.StateNode, *ast.SubstateMember, *ast.PseudostateNode, *ast.InitialNode, *ast.FinalNode:
		return true
	case *ast.Usage:
		return d.Kind == ast.UsageState
	}
	return false
}

// machineScope returns the scope of the machine around scope: the outermost
// state, region or machine body enclosing it.
func machineScope(scope *symbols.Scope) *symbols.Scope {
	machine := scope
	for s := scope; s != nil; s = s.Parent() {
		if !stateBody(s.Node()) {
			break
		}
		machine = s
	}
	return machine
}

// stateBody reports whether node's body belongs to a state machine: the machine
// itself, one of its states, a region, or a transition declared in one.
func stateBody(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.StateNode, *ast.SubstateMember, *ast.StateRegion, *ast.PseudostateNode, *ast.TransitionMember:
		return true
	case *ast.Usage:
		return n.Kind == ast.UsageState || n.Kind == ast.UsageTransition
	case *ast.Definition:
		return n.Kind == ast.DefState
	}
	return false
}

// firstVertex searches scope's subtree, outermost and in declaration order, for
// a vertex whose name path ends in parts.
func firstVertex(scope *symbols.Scope, parts []string) (*symbols.Symbol, bool) {
	if scope == nil || len(parts) == 0 {
		return nil, false
	}
	name := parts[len(parts)-1]
	for _, key := range scope.MemberNames() {
		if key != name {
			continue
		}
		if !ownedBy(scope, parts[:len(parts)-1]) {
			continue
		}
		for _, sym := range symbols.PreferDeclared(scope.LookupLocalAll(key)) {
			if isVertex(sym.Decl) {
				return sym, true
			}
		}
	}
	for _, child := range scope.Children() {
		if sym, ok := firstVertex(child, parts); ok {
			return sym, true
		}
	}
	return nil, false
}

// ownedBy reports whether the scopes enclosing scope are named by owners,
// innermost last: `outer::inner` need not name the scopes between them.
func ownedBy(scope *symbols.Scope, owners []string) bool {
	remaining := len(owners)
	for s := scope; s != nil && remaining > 0; s = s.Parent() {
		if scopeName(s) == owners[remaining-1] {
			remaining--
		}
	}
	return remaining == 0
}

// scopeName is the name of the declaration owning scope, empty when it has none.
func scopeName(scope *symbols.Scope) string {
	if owner := scope.Owner(); owner != nil {
		return owner.Name
	}
	return ast.SimpleName(scope.Node())
}

// vertexNames lists the distinct names of the vertices declared in scope's
// subtree: a name declared in two regions is one suggestion, not two.
func vertexNames(scope *symbols.Scope) []string {
	var names []string
	seen := map[string]bool{}
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		for _, key := range scope.MemberNames() {
			for _, sym := range scope.LookupLocalAll(key) {
				if isVertex(sym.Decl) && !seen[key] {
					seen[key] = true
					names = append(names, key)
					break
				}
			}
		}
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(scope)
	return names
}

// qualifiedParts is the segments of a qualified name as written.
func qualifiedParts(qn *ast.QualifiedName) []string {
	parts := make([]string, len(qn.Parts))
	for i, part := range qn.Parts {
		parts[i] = part.Text
	}
	return parts
}
