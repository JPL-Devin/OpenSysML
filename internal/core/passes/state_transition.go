package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CodeEndpointNotOfMachine marks a transition endpoint naming a vertex the
// machine the transition belongs to does not own (UML 2.5.1 §14.2.3.9).
const CodeEndpointNotOfMachine = "endpoint-not-of-machine"

// CodeNoOutgoingTransition marks a routing pseudostate no transition leaves,
// which a transition reaching it terminates nowhere at (UML 2.5.1 §15.7.18).
const CodeNoOutgoingTransition = "no-outgoing-transition"

// CodeParallelStateTransition marks a succession or transition owned by a
// parallel state, whose substates are concurrent (SysML v2 §7.16).
const CodeParallelStateTransition = "parallel-state-transition"

// msgParallelStateTransition is the pilot's wording of the same rule.
const msgParallelStateTransition = "A parallel state cannot have successions or transitions."

// CodeAccepterSourceNotState marks a transition whose accepter waits in
// something other than a state (SysML v2 §7.16 TransitionUsage).
const CodeAccepterSourceNotState = "accepter-source-not-state"

// msgAccepterSourceNotState is the pilot's wording of the same rule.
const msgAccepterSourceNotState = "A transition with an accepter must have a state as its source."

// StateTransitionPass checks that every transition names one source and one
// target vertex of its own machine (UML 2.5.1 §14.2.3.9), and that a routing
// pseudostate is left by one (§15.7.18).
type StateTransitionPass struct{}

// Level reports the name-resolution level: resolved endpoints are all it reads.
func (StateTransitionPass) Level() PassLevel { return LevelNameResolution }

// Run checks every state machine the document declares.
func (StateTransitionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	checker := &transitionChecker{resolver: ctx.Resolver()}
	checker.findMachines(rootScope, root.Members)
	return checker.diags
}

// transitionChecker accumulates the diagnostics of one document.
type transitionChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

// machine holds what checking one state machine needs: the vertices it owns,
// those its transitions leave, and its routing pseudostates in source order.
type machine struct {
	vertices   map[ast.Node]bool
	sources    map[ast.Node]bool
	unresolved map[string]bool
	routing    []*ast.PseudostateNode
}

// findMachines walks the document for state machine declarations. A state inside
// another state is one of its vertices, so machine bodies are not searched.
func (c *transitionChecker) findMachines(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		decl := unwrapMembership(member)
		child := bodyScope(scope, decl)
		switch n := decl.(type) {
		case *ast.Package:
			c.findMachines(child, n.Members)
		case *ast.Namespace:
			c.findMachines(child, n.Members)
		case *ast.Definition:
			if n.Kind == ast.DefState {
				c.checkParallelStates(n.Members, n.IsParallel)
				c.checkMachine(n, child)
				continue
			}
			c.findMachines(child, n.Members)
		case *ast.Usage:
			if n.Kind == ast.UsageState {
				c.checkParallelStates(n.Members, n.IsParallel)
				c.checkMachine(n, child)
				continue
			}
			c.findMachines(child, n.Members)
		}
	}
}

// checkMachine checks the transitions of one machine against the vertices it
// owns, then reports the routing pseudostates none of them leaves.
func (c *transitionChecker) checkMachine(decl ast.Node, scope *symbols.Scope) {
	// A machine whose vertices do not collect is one lowering reports about, and
	// checking endpoints against a partial set would report legal ones.
	vertices, err := lower.VertexDecls(decl, scope)
	if err != nil {
		return
	}
	m := &machine{
		vertices:   vertices,
		sources:    map[ast.Node]bool{},
		unresolved: map[string]bool{},
	}
	c.walkBody(m, scope, declMembers(decl), decl)

	for _, ps := range m.routing {
		if m.sources[ps] || m.unresolved[ps.Name] {
			continue
		}
		c.report(ps.Span(), CodeNoOutgoingTransition, fmt.Sprintf(
			"%s %s has no outgoing transition, so a transition reaching it terminates nowhere",
			ps.Kind, ps.Name))
	}
}

// checkParallelStates reports the successions and transitions a parallel state
// owns, orthogonality being each state's own (SysML v2 §7.16, isParallel).
func (c *transitionChecker) checkParallelStates(members []ast.Node, parallel bool) {
	for _, member := range members {
		decl := unwrapMembership(member)
		if parallel && parallelStateOrdering(decl) {
			c.report(decl.Span(), CodeParallelStateTransition, msgParallelStateTransition)
		}
		switch n := decl.(type) {
		case *ast.Definition:
			if n.Kind == ast.DefState {
				c.checkParallelStates(n.Members, n.IsParallel)
			}
		case *ast.Usage:
			if n.Kind == ast.UsageState {
				c.checkParallelStates(n.Members, n.IsParallel)
			}
		case *ast.StateNode:
			c.checkParallelStates(n.Substates, false)
			for _, region := range n.Regions {
				c.checkParallelStates(region.States, false)
			}
		case *ast.StateRegion:
			c.checkParallelStates(n.States, false)
		}
	}
}

// checkAccepterSource reports a transition whose accepter names a source that is
// not a state, and says whether it did. An accepter waits while its source is
// performed, and only a state is performed until something leaves it, so a
// transition with a trigger leaves a state (SysML v2 §7.16 TransitionUsage).
func (c *transitionChecker) checkAccepterSource(
	scope *symbols.Scope,
	n *ast.TransitionMember,
) bool {
	if n.Trigger == nil {
		return false
	}
	sym, ok := c.resolver.EndpointSymbol(scope, n.Source)
	if !ok || !isPerformedAction(sym.Decl) {
		return false
	}
	at := n.TriggerSpan
	if at.Len == 0 {
		at = n.Trigger.Span()
	}
	c.report(at, CodeAccepterSourceNotState, msgAccepterSourceNotState)
	return true
}

// isPerformedAction reports whether a declaration a transition endpoint reached
// is an action, which completes on its own rather than waiting in a state.
func isPerformedAction(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.Usage:
		return n.Kind == ast.UsageAction
	case *ast.Definition:
		return n.Kind == ast.DefAction
	}
	return false
}

// parallelStateOrdering reports whether a state body member orders the states
// around it, in any of the notations a succession or transition is written in.
func parallelStateOrdering(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.SuccessionEdge, *ast.TransitionMember, *ast.TransitionEdge:
		return true
	case *ast.InitialNode:
		// `first s1 then s2;` is a succession whose source the marker names.
		return n.Successor != nil
	case *ast.Usage:
		return n.Kind == ast.UsageSuccession || n.Kind == ast.UsageTransition
	}
	return false
}

// walkBody collects the transitions and routing pseudostates of a machine body,
// descending into its states and regions with the scope each was declared in.
// owner is the declaration whose body this is, whose entry actions a transition
// written here may leave — the same body lowering reads them from.
func (c *transitionChecker) walkBody(m *machine, scope *symbols.Scope, members []ast.Node, owner ast.Node) {
	starts := map[ast.Node]bool{}
	for _, action := range ast.EntryActions(members) {
		starts[action] = true
	}
	for _, action := range ast.StateEntryActions(owner) {
		starts[action] = true
	}
	for _, member := range members {
		decl := unwrapMembership(member)
		switch n := decl.(type) {
		case *ast.TransitionMember:
			// A sourceless `accept … then` takes the state it is written in as its
			// source (SysML 7.19.3), which names a vertex by construction.
			if n.Source != nil {
				if c.checkAccepterSource(scope, n) {
					c.checkEndpoint(m, scope, n.Target, true, nil)
					continue
				}
				bare := n.Trigger == nil && n.Guard == nil && len(n.Effect) == 0
				m.markLeft(c.checkEndpoint(m, scope, n.Source, false, c.startsOf(m, scope, n.Target, bare, starts)), n.Source)
			}
			c.checkEndpoint(m, scope, n.Target, true, nil)
		case *ast.SuccessionEdge:
			// `off then busy;`, whose source is elided by the `entry; then off;` form.
			if n.Source != nil {
				m.markLeft(c.checkEndpoint(m, scope, n.Source, false, c.startsOf(m, scope, n.Target, true, starts)), n.Source)
			}
			c.checkEndpoint(m, scope, n.Target, true, nil)
		case *ast.TransitionEdge:
			m.markLeft(c.checkEndpoint(m, scope, n.Source, false, nil), n.Source)
			c.checkEndpoint(m, scope, n.Target, true, nil)
		case *ast.InitialNode:
			// The marker's `then` is its one outgoing transition.
			if n.Successor != nil {
				c.checkEndpoint(m, scope, n.Successor, true, nil)
			}
		case *ast.PseudostateNode:
			if routingPseudostate(n.Kind) {
				m.routing = append(m.routing, n)
			}
		case *ast.StateNode:
			c.walkBody(m, bodyScope(scope, n), n.Substates, n)
			for _, region := range n.Regions {
				c.walkBody(m, bodyScope(scope, region), region.States, nil)
			}
		case *ast.StateRegion:
			c.walkBody(m, bodyScope(scope, n), n.States, nil)
		case *ast.Usage:
			switch n.Kind {
			case ast.UsageState:
				c.walkBody(m, bodyScope(scope, n), n.Members, n)
			case ast.UsageSuccession:
				// `a then b;`, written as a connector whose two ends name vertices.
				if len(n.ConnectorEnds) == 2 {
					source := connectorEndName(n.ConnectorEnds[0])
					target := connectorEndName(n.ConnectorEnds[1])
					m.markLeft(c.checkEndpoint(m, scope, source, false, c.startsOf(m, scope, target, true, starts)), source)
					c.checkEndpoint(m, scope, target, true, nil)
				}
			}
		}
	}
}

// startsOf returns the entry actions a transition of this shape may leave: only a
// bare completion transition into a state names the state the machine starts in,
// which is the shape lowering reads (SysML 7.19.3).
func (c *transitionChecker) startsOf(
	m *machine,
	scope *symbols.Scope,
	target *ast.QualifiedName,
	bare bool,
	starts map[ast.Node]bool,
) map[ast.Node]bool {
	if !bare || target == nil {
		return nil
	}
	sym, ok := c.resolver.EndpointSymbol(scope, target)
	if !ok {
		return nil
	}
	decl := sym.Decl
	if !m.vertices[decl] && !resolve.IsInheritedStateVertex(sym) {
		return nil
	}
	if _, pseudostate := decl.(*ast.PseudostateNode); pseudostate {
		return nil
	}
	return starts
}

// checkEndpoint reports an endpoint naming something no transition of this
// machine may reach, and returns what it named. An unresolved one is left to
// name resolution. starts are the entry actions this source may leave, each
// standing in for a start pseudostate.
func (c *transitionChecker) checkEndpoint(
	m *machine,
	scope *symbols.Scope,
	qn *ast.QualifiedName,
	isTarget bool,
	starts map[ast.Node]bool,
) ast.Node {
	if qn == nil {
		return nil
	}
	sym, ok := c.resolver.EndpointSymbol(scope, qn)
	if !ok {
		return nil
	}
	decl := sym.Decl
	if m.vertices[decl] || resolve.IsInheritedStateVertex(sym) ||
		c.resolver.MachineStateVertex(scope, qn, sym) {
		return decl
	}
	// A `first m then x` marker gets no incoming transition (UML 15.7.18), so a
	// marker target is illegal; a transition out of one is left to lowering.
	if isMarker(decl) && !isTarget {
		return decl
	}
	// A transition out of the body's entry action says which state the machine
	// starts in, the action standing in for a start pseudostate (SysML 7.19.3).
	if starts[decl] && !isTarget {
		return decl
	}
	c.report(qn.Span(), CodeEndpointNotOfMachine, fmt.Sprintf(
		lower.NotAVertexFormat, endpointText(qn), lower.VertexKind(decl)))
	return decl
}

// report records one diagnostic of this pass.
func (c *transitionChecker) report(span source.Span, code, message string) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  message,
		Code:     code,
		Source:   "state-transition",
	})
}

// markLeft records the vertex a transition leaves by declaration, so sibling
// regions declaring same-named pseudostates do not mask each other's dead ends.
// An endpoint naming no vertex is recorded by name instead: what it meant to
// leave is unknown, and reporting that as a dead end would be a false positive.
func (m *machine) markLeft(decl ast.Node, qn *ast.QualifiedName) {
	if decl != nil {
		m.sources[decl] = true
		return
	}
	if qn != nil && len(qn.Parts) > 0 {
		m.unresolved[qn.Parts[len(qn.Parts)-1].Text] = true
	}
}

// isMarker reports whether decl is a `first`/`then` marker rather than a state
// or pseudostate: what an action body's control flow is written with.
func isMarker(decl ast.Node) bool {
	switch decl.(type) {
	case *ast.InitialNode, *ast.FinalNode:
		return true
	}
	return false
}

// routingPseudostate reports whether a pseudostate only routes onward. History,
// entry and exit points are excluded: what they reach needs no transition.
func routingPseudostate(kind ast.PseudostateKind) bool {
	switch kind {
	case ast.PseudostateChoice, ast.PseudostateJunction, ast.PseudostateFork, ast.PseudostateJoin:
		return true
	}
	return false
}

// connectorEndName is the name a succession's end references.
func connectorEndName(end *ast.ConnectorEnd) *ast.QualifiedName {
	if end == nil {
		return nil
	}
	if qn := ast.AsQualifiedName(end.Target); qn != nil {
		return qn
	}
	return ast.AsQualifiedName(end.Reference)
}

// endpointText renders an endpoint name as written, for a message about it.
func endpointText(qn *ast.QualifiedName) string {
	parts := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}

// unwrapMembership strips the membership a declaration reaches a body wrapped in.
func unwrapMembership(node ast.Node) ast.Node {
	if membership, ok := node.(*ast.Membership); ok {
		return membership.Member
	}
	return node
}

// bodyScope returns the scope decl declares into, or scope itself when the
// scope builder gave it none.
func bodyScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	if scope == nil || decl == nil {
		return scope
	}
	if child := scope.ChildFor(decl); child != nil {
		return child
	}
	return scope
}

// declMembers is the body of a definition or usage declaration.
func declMembers(decl ast.Node) []ast.Node {
	switch n := decl.(type) {
	case *ast.Definition:
		return n.Members
	case *ast.Usage:
		return n.Members
	}
	return nil
}
