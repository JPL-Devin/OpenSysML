package lower

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// StateGraph is the execution IR for state machines.
type StateGraph struct {
	// Scope is the scope the machine's own body was declared in, in which the
	// expressions written directly among its members resolve their names.
	Scope *symbols.Scope

	// Attributes are the attribute defaults the machine declares, in declaration
	// order: the values its state data starts with.
	Attributes []Attribute

	// StateScopes: state → the scope that state's body was declared in, which is
	// what the names in its entry, do and exit behaviors resolve against.
	StateScopes map[*ast.StateNode]*symbols.Scope

	// Behaviors: state → its lowered entry, do and exit behaviors. The executor
	// runs these rather than the state's AST members, so an inline action body is
	// executable statements by the time it is reached.
	Behaviors map[*ast.StateNode]*StateBehaviors

	// declOf: synthesized state → the declaration it was built from, since the
	// scope tree is keyed by what the scope builder saw rather than by the state
	// nodes lowering derives from it.
	declOf map[*ast.StateNode]ast.Node

	// vertexOf: the declaration an endpoint resolves to → the graph node standing
	// for it, the state node built from it or the pseudostate itself.
	vertexOf map[ast.Node]ast.Node

	// endpoints resolves what a transition endpoint names.
	endpoints Endpoints

	// States in the machine (flat list, includes nested)
	States []*ast.StateNode

	// Pseudostates (choice, junction, etc.)
	Pseudostates map[string]*ast.PseudostateNode

	// PseudostateOwner: pseudostate -> the composite state that declares it,
	// absent for one declared directly in the machine. A history pseudostate
	// restores the configuration of its owner, so the owner must survive lowering.
	PseudostateOwner map[*ast.PseudostateNode]*ast.StateNode

	// Transitions: source node (StateNode or PseudostateNode) → list of transitions
	Transitions map[ast.Node][]*Transition

	// CompositeStates: state → regions
	CompositeStates map[*ast.StateNode][]*ast.StateRegion

	// RegionInitials: region → initial state
	RegionInitials map[*ast.StateRegion]*ast.StateNode

	// ParentState: child → parent
	ParentState map[*ast.StateNode]*ast.StateNode

	// RegionOwner: region → owning composite state
	RegionOwner map[*ast.StateRegion]*ast.StateNode

	// Deferred: state → the triggers it defers while active, normalized the same
	// way transition triggers are.
	Deferred map[*ast.StateNode][]ast.Node

	// TopRegions are the machine's own orthogonal regions, in declaration order.
	// The order is observable: it is the order regions are entered and exited in.
	TopRegions []*ast.StateRegion

	// RegionOf: state → the region that declares it (fork/join need the region a
	// target belongs to)
	RegionOf map[*ast.StateNode]*ast.StateRegion

	// InitialState (required for simple machines, nil for multi-region)
	Initial *ast.StateNode

	// Connections are the connectors declared in the state machine body, which
	// is how a `send ... via <port>` in an entry/do/exit/effect action finds the
	// ports it reaches.
	Connections []Connection
}

// Transition represents a state transition (lowered from TransitionEdge or TransitionMember).
type Transition struct {
	// Name is the transition's own name, when it was written with one
	// (`transition maintain first idle then busy`), and "" when it was not.
	Name    string
	Source  ast.Node // *ast.StateNode or *ast.PseudostateNode
	Target  ast.Node // *ast.StateNode or *ast.PseudostateNode
	Trigger ast.Node // TimeEvent, ChangeEvent, SignalEvent, CallEvent, nil = completion
	Guard   ast.Node // guard expression, nil = no guard
	// Effect are the transition's effect behaviors, lowered the same way a state's
	// entry, do and exit behaviors are.
	Effect []StateBehavior
	// Via is the port the accepted occurrence must arrive at
	// (`accept Ping via commPort`), and "" when the trigger names no port, in
	// which case an occurrence reaching the machine by any route fires it.
	Via string

	// Scope is the scope the transition was declared in, in which the expressions
	// its trigger carries — a time event's duration, a change event's condition —
	// resolve their names.
	Scope *symbols.Scope

	// BodyScope is the scope the transition's guard and effect resolve in. It is
	// Scope, except for a call trigger, whose parameters are visible to the guard
	// and effect and nowhere else (`accept setSpeed(v) if v > 0`).
	BodyScope *symbols.Scope
}

// ToStateGraph converts a state machine AST (Usage or Definition) to a StateGraph.
// scope is the scope the machine's body was declared in — the scope the machine
// itself owns — from which the scope of every state and transition it carries is
// derived. Its endpoints resolve against the machine's own symbols; a caller
// holding the name-resolution tier's uses ToStateGraphWithEndpoints instead.
func ToStateGraph(stateMachineDecl ast.Node, scope *symbols.Scope) (*StateGraph, error) {
	return ToStateGraphWithEndpoints(stateMachineDecl, scope, nil)
}

// ToStateGraphWithEndpoints lowers a state machine, building its transitions
// from the endpoints the name-resolution tier already resolved.
func ToStateGraphWithEndpoints(stateMachineDecl ast.Node, scope *symbols.Scope, endpoints Endpoints) (*StateGraph, error) {
	// A machine no scope tree holds is indexed on its own; without the tier's
	// resolver, one that has a tree names its endpoints from that tree alone.
	switch {
	case scope == nil:
		endpoints = localEndpoints(stateMachineDecl)
	case endpoints == nil:
		endpoints = scopeEndpoints{machine: scope}
	}
	graph := newStateGraph(scope, endpoints)

	members, err := machineMembers(stateMachineDecl)
	if err != nil {
		return nil, err
	}

	graph.Connections = lowerConnections(members, OwnerBehavior, scope)
	graph.Attributes = lowerAttributes(members)

	if err := collectVertices(graph, members, scope); err != nil {
		return nil, err
	}

	// Second pass: identify composite states with regions AND handle top-level regions
	// Check for top-level regions (state machine itself has regions as members)
	hasTopLevelRegions := false
	for _, member := range members {
		actualMember := unwrapMembership(member)
		if region, ok := actualMember.(*ast.StateRegion); ok {
			hasTopLevelRegions = true
			graph.TopRegions = append(graph.TopRegions, region)
			if graph.RegionInitials[region] == nil {
				return nil, fmt.Errorf("top-level region %s has no initial state", region.Name)
			}
		}
	}

	// Also handle states that have regions as sub-members
	for _, state := range graph.States {
		if len(state.Regions) > 0 {
			graph.CompositeStates[state] = state.Regions

			for _, region := range state.Regions {
				graph.RegionOwner[region] = state

				if graph.RegionInitials[region] == nil {
					return nil, fmt.Errorf("region %s in state %s has no initial state", region.Name, state.Name)
				}
			}
		}
	}

	// Record the triggers each state defers, once every state is collected.
	for _, state := range graph.States {
		if err := collectDeferred(graph, state); err != nil {
			return nil, err
		}
	}

	// Third pass: collect transitions
	if err := collectTransitions(graph, members, nil, nil, scope); err != nil {
		return nil, err
	}

	// Find initial state (for simple machines - machines without top-level regions).
	// Regions nested inside a composite state do not remove the machine's own
	// initial state.
	if !hasTopLevelRegions {
		// Find the leaf initial state (deepest in a chain of initials)
		// When multiple initial chains exist in different branches, prefer the shallowest branch root
		var selectedInitial *ast.StateNode
		minRootDepth := 999999
		maxLeafDepth := -1

		for _, state := range graph.States {
			if !state.IsInitial {
				continue
			}

			// Calculate depth (distance from root)
			depth := 0
			current := state
			for {
				parent, hasParent := graph.ParentState[current]
				if !hasParent {
					break
				}
				depth++
				current = parent
			}

			// Find the root of the initial chain (topmost initial ancestor)
			chainRoot := state
			for {
				parent, hasParent := graph.ParentState[chainRoot]
				if !hasParent || !parent.IsInitial {
					break
				}
				chainRoot = parent
			}

			// Calculate chain root depth
			rootDepth := 0
			current = chainRoot
			for {
				parent, hasParent := graph.ParentState[current]
				if !hasParent {
					break
				}
				rootDepth++
				current = parent
			}

			// Prefer chain with shallowest root, then deepest leaf within that chain
			if rootDepth < minRootDepth || (rootDepth == minRootDepth && depth > maxLeafDepth) {
				selectedInitial = state
				minRootDepth = rootDepth
				maxLeafDepth = depth
			}
		}

		// Only set Initial if there are no top-level regions
		// (executor will use RegionInitials for orthogonal region machines)
		if selectedInitial != nil {
			graph.Initial = selectedInitial
		}
	}
	// If there are top-level regions, Initial stays nil (executor will use RegionInitials instead)

	// Note: Initial state is optional at graph construction time.
	// The executor's initialize() will validate and return the error if missing.
	// Top-level regions are also valid (no single initial state).

	return graph, nil
}

// stateNodeFromUsage builds the state node for `state <name> { ... }`, carrying
// over the entry/do/exit behaviors declared in the body so the executor runs
// them; without this the body's behaviors are silently dropped. The usage it was
// built from is recorded, since that is the declaration the scope tree is keyed
// by.
func stateNodeFromUsage(graph *StateGraph, usage *ast.Usage) *ast.StateNode {
	name, _ := ast.EffectiveName(usage)
	state := &ast.StateNode{Name: name}
	state.NodeSpan = usage.NodeSpan
	graph.declOf[state] = usage

	for _, member := range usage.Members {
		switch m := unwrapMembership(member).(type) {
		case *ast.EntryMember:
			state.Entry = append(state.Entry, m.Actions...)
		case *ast.DoMember:
			state.Do = append(state.Do, m.Actions...)
		case *ast.ExitMember:
			state.Exit = append(state.Exit, m.Actions...)
		case *ast.DeferMember:
			state.Defer = append(state.Defer, m.Triggers...)
		case *ast.StateRegion:
			state.Regions = append(state.Regions, m)
		case *ast.StateNode:
			state.Substates = append(state.Substates, m)
		case *ast.PseudostateNode:
			state.Substates = append(state.Substates, m)
		case *ast.SubstateMember:
			child := &ast.StateNode{Name: m.Name}
			child.NodeSpan = m.NodeSpan
			graph.declOf[child] = m
			state.Substates = append(state.Substates, child)
		case *ast.Usage:
			// A state declared inside a composite state is one of its substates:
			// dropping it here would leave the hierarchy out of the graph and every
			// transition naming a nested state unresolvable.
			if m.Kind == ast.UsageState {
				state.Substates = append(state.Substates, stateNodeFromUsage(graph, m))
			}
		}
	}
	return state
}

// newStateGraph is an empty graph of a machine whose body scope is scope and
// whose endpoints resolve through endpoints.
func newStateGraph(scope *symbols.Scope, endpoints Endpoints) *StateGraph {
	return &StateGraph{
		Scope:            scope,
		vertexOf:         make(map[ast.Node]ast.Node),
		endpoints:        endpoints,
		StateScopes:      make(map[*ast.StateNode]*symbols.Scope),
		Behaviors:        make(map[*ast.StateNode]*StateBehaviors),
		declOf:           make(map[*ast.StateNode]ast.Node),
		States:           make([]*ast.StateNode, 0),
		Pseudostates:     make(map[string]*ast.PseudostateNode),
		PseudostateOwner: make(map[*ast.PseudostateNode]*ast.StateNode),
		Transitions:      make(map[ast.Node][]*Transition),
		CompositeStates:  make(map[*ast.StateNode][]*ast.StateRegion),
		RegionInitials:   make(map[*ast.StateRegion]*ast.StateNode),
		ParentState:      make(map[*ast.StateNode]*ast.StateNode),
		RegionOwner:      make(map[*ast.StateRegion]*ast.StateNode),
		RegionOf:         make(map[*ast.StateNode]*ast.StateRegion),
		Deferred:         make(map[*ast.StateNode][]ast.Node),
	}
}

// machineMembers is the body of a state machine declaration.
func machineMembers(stateMachineDecl ast.Node) ([]ast.Node, error) {
	switch n := stateMachineDecl.(type) {
	case *ast.Usage:
		return n.Members, nil
	case *ast.Definition:
		return n.Members, nil
	}
	return nil, fmt.Errorf("state machine must be Usage or Definition, got %T", stateMachineDecl)
}

// collectVertices records every state and pseudostate the machine body declares,
// which are the vertices its transitions may name (UML 2.5.1 §14.2.3.9).
func collectVertices(graph *StateGraph, members []ast.Node, scope *symbols.Scope) error {
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.StateNode:
			if err := collectStates(graph, n, nil, graph.stateScope(scope, n)); err != nil {
				return err
			}
		case *ast.Usage:
			// `state <name> { … }`, parsed as a usage rather than a state node.
			if n.Kind == ast.UsageState {
				// The state node records the usage it came from, so its scope is the
				// one that usage declares: build it before asking for the scope.
				state := stateNodeFromUsage(graph, n)
				if err := collectStates(graph, state, nil, graph.stateScope(scope, state)); err != nil {
					return err
				}
			}
		case *ast.SubstateMember:
			// `state <name>;`, which declares a state with no body.
			stateNode := &ast.StateNode{Name: n.Name}
			stateNode.NodeSpan = n.NodeSpan
			graph.declOf[stateNode] = n
			if err := collectStates(graph, stateNode, nil, graph.stateScope(scope, stateNode)); err != nil {
				return err
			}
		case *ast.StateRegion:
			// Top-level region: collect its states, which no state is the parent of
			if err := collectRegionStates(graph, n, nil, childScope(scope, n)); err != nil {
				return err
			}
		case *ast.PseudostateNode:
			graph.addPseudostate(n)
		case *ast.DeferMember:
			// The machine's own body has no state to defer for: an event deferred
			// there would be retained for the whole run and never redelivered.
			return fmt.Errorf("defer must be declared inside a state, not in the state machine body")
		}
	}
	return nil
}

// collectStates recursively collects states and builds parent relationships.
// scope is the scope the state's own body was declared in, from which the scope
// of each of its substates and regions is derived.
func collectStates(graph *StateGraph, state *ast.StateNode, parent *ast.StateNode, scope *symbols.Scope) error {
	graph.States = append(graph.States, state)
	graph.addVertex(state)
	graph.StateScopes[state] = scope
	graph.Behaviors[state] = &StateBehaviors{
		Entry: LowerBehaviors(state.Entry, scope),
		Do:    LowerBehaviors(state.Do, scope),
		Exit:  LowerBehaviors(state.Exit, scope),
	}
	if parent != nil {
		graph.ParentState[state] = parent
	}

	// Recursively collect substates
	for _, substate := range state.Substates {
		switch child := unwrapMembership(substate).(type) {
		case *ast.StateNode:
			if err := collectStates(graph, child, state, graph.stateScope(scope, child)); err != nil {
				return err
			}
		case *ast.PseudostateNode:
			// A pseudostate declared inside a composite state belongs to it: that
			// ownership is what a history pseudostate restores from, and without it
			// a nested pseudostate is not part of the graph at all.
			graph.addPseudostate(child)
			graph.PseudostateOwner[child] = state
		}
	}

	// Collect states in orthogonal regions
	for _, region := range state.Regions {
		if err := collectRegionStates(graph, region, state, childScope(scope, region)); err != nil {
			return err
		}
	}
	return nil
}

// stateScope returns the scope state's body was declared in, given the scope of
// the body that declares it. A state lowering synthesized from a usage or a bare
// substate declaration is keyed in the scope tree by that declaration.
func (g *StateGraph) stateScope(parent *symbols.Scope, state *ast.StateNode) *symbols.Scope {
	decl := ast.Node(state)
	if origin, ok := g.declOf[state]; ok {
		decl = origin
	}
	return childScope(parent, decl)
}

// collectRegionStates collects the states an orthogonal region declares, records
// which region declares each of them and which one the region starts in, and
// assigns the region's pseudostates to the state that owns the region. parent is
// the state owning the region, nil for the machine's own regions.
//
// Region members reach here as a state node, a bare `state <name>;` substate or a
// state usage with a body, each of them possibly wrapped in a membership: a state
// missed here is a state no transition can name.
func collectRegionStates(graph *StateGraph, region *ast.StateRegion, parent *ast.StateNode, scope *symbols.Scope) error {
	for _, member := range region.States {
		var state *ast.StateNode
		switch n := unwrapMembership(member).(type) {
		case *ast.StateNode:
			state = n
		case *ast.SubstateMember:
			state = &ast.StateNode{Name: n.Name}
			state.NodeSpan = n.NodeSpan
			graph.declOf[state] = n
		case *ast.Usage:
			if n.Kind != ast.UsageState {
				continue
			}
			state = stateNodeFromUsage(graph, n)
		case *ast.PseudostateNode:
			graph.addPseudostate(n)
			if parent != nil {
				graph.PseudostateOwner[n] = parent
			}
			continue
		case *ast.DeferMember:
			// A region is not a state: only a state can retain an event.
			return fmt.Errorf("defer must be declared inside a state, not in a region body")
		default:
			continue
		}

		if err := collectStates(graph, state, parent, graph.stateScope(scope, state)); err != nil {
			return err
		}
		graph.RegionOf[state] = region
		if state.IsInitial && graph.RegionInitials[region] == nil {
			graph.RegionInitials[region] = state
		}
	}
	return nil
}

// collectDeferred records the triggers a state defers, normalized the same way
// transition triggers are. Only a signal or a call can be deferred: a time or
// change event is not dispatched from the event pool, so retaining one has no
// meaning and is reported rather than silently ignored.
func collectDeferred(graph *StateGraph, state *ast.StateNode) error {
	for _, trigger := range state.Defer {
		if trigger == nil {
			return fmt.Errorf("state %s defers a nil trigger", state.Name)
		}
		switch typed := classifyTrigger(trigger).(type) {
		case *ast.AcceptEvent:
			if ast.SimpleName(typed.SignalType) == "" {
				return fmt.Errorf("state %s defers a signal trigger that names no signal", state.Name)
			}
			graph.Deferred[state] = append(graph.Deferred[state], typed)
		case *ast.CallEvent:
			graph.Deferred[state] = append(graph.Deferred[state], typed)
		default:
			return fmt.Errorf("state %s defers a %T trigger: only signal and call triggers can be deferred", state.Name, typed)
		}
	}
	return nil
}

// findStateByName looks up a state by qualified name.
func findStateByName(states []*ast.StateNode, qname *ast.QualifiedName) *ast.StateNode {
	if qname == nil || len(qname.Parts) == 0 {
		return nil
	}

	targetName := qname.Parts[len(qname.Parts)-1].Text
	for _, state := range states {
		if state.Name == targetName {
			return state
		}
	}
	return nil
}

// addVertex records the graph node standing for a state, and for the
// declaration it was built from, which is what an endpoint resolves to.
func (g *StateGraph) addVertex(state *ast.StateNode) {
	g.vertexOf[state] = state
	if decl, ok := g.declOf[state]; ok {
		g.vertexOf[decl] = state
	}
}

// addPseudostate records a pseudostate as a vertex of the graph.
func (g *StateGraph) addPseudostate(ps *ast.PseudostateNode) {
	g.Pseudostates[ps.Name] = ps
	g.vertexOf[ps] = ps
}

// vertex is the graph node a transition endpoint names: name resolution says
// which declaration it reaches, and lowering collected a vertex from that. An
// endpoint naming no vertex yields a nil node and no error — the name-resolution
// tier reports it, and how strictly the machine then runs is the runtime's call.
func (g *StateGraph) vertex(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, error) {
	if qn == nil {
		return nil, fmt.Errorf("transition endpoint names nothing")
	}
	decl, ok := g.endpoints.Endpoint(scope, qn)
	if !ok {
		return nil, nil
	}
	node, ok := g.vertexOf[decl]
	if !ok {
		return nil, fmt.Errorf(NotAVertexFormat, endpointText(qn), VertexKind(decl))
	}
	return node, nil
}

// NotAVertexFormat reports an endpoint naming an element outside the machine's
// vertices, shared by the check reporting it and the lowering backstopping it.
const NotAVertexFormat = "transition endpoint %s names a %s that is not a vertex of this state machine"

// VertexKind names what an endpoint reached in modelling terms, for a message a
// modeller reads.
func VertexKind(decl ast.Node) string {
	switch decl.(type) {
	case *ast.StateNode, *ast.SubstateMember, *ast.Usage:
		return "state"
	case *ast.PseudostateNode:
		return "pseudostate"
	case *ast.InitialNode:
		return "start marker"
	case *ast.FinalNode:
		return "end marker"
	}
	return "element"
}

// endpointText renders an endpoint name for an error message.
func endpointText(qn *ast.QualifiedName) string {
	parts := make([]string, 0, len(qn.Parts))
	for _, p := range qn.Parts {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "::")
}

// lowerTransitionEdge converts a TransitionEdge (legacy) to a Transition. No
// document declares such an edge, so an endpoint naming no vertex reports here.
// scope is the scope the edge was declared in.
func lowerTransitionEdge(graph *StateGraph, edge *ast.TransitionEdge, scope *symbols.Scope) (*Transition, error) {
	source, err := graph.vertex(scope, edge.Source)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("transition edge references undefined source state %s", endpointText(edge.Source))
	}
	target, err := graph.vertex(scope, edge.Target)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("transition edge references undefined target state %s", endpointText(edge.Target))
	}

	return &Transition{
		Source:    source,
		Target:    target,
		Trigger:   edge.Trigger,
		Guard:     edge.Guard,
		Effect:    LowerBehaviors(edge.Effect, scope),
		Scope:     scope,
		BodyScope: scope,
	}, nil
}

// lowerTransitionMember converts a TransitionMember (parser output) to a Transition.
// containingState is used as the source when member.Source is nil (sourceless accept...then).
// scope is the scope the transition was declared in.
func lowerTransitionMember(graph *StateGraph, member *ast.TransitionMember, containingState ast.Node, scope *symbols.Scope) (*Transition, error) {
	// A sourceless `accept ... then` leaves the state it is written in, so the
	// state declaring it is the source; anywhere else it names no source at all.
	var source ast.Node
	if member.Source == nil {
		if containingState == nil {
			return nil, fmt.Errorf("sourceless transition (accept...then) at top level has no containing state")
		}
		vertex, ok := graph.vertexOf[containingState]
		if !ok {
			return nil, fmt.Errorf("sourceless transition is declared in a %T that is not a state of the machine", containingState)
		}
		source = vertex
	} else {
		vertex, err := graph.vertex(scope, member.Source)
		if err != nil || vertex == nil {
			return nil, err
		}
		source = vertex
	}

	// A transition the parser could not read a target from names no edge.
	if member.Target == nil {
		return nil, fmt.Errorf("transition %s names no target", orAnonymous(member.Name))
	}

	target, err := graph.vertex(scope, member.Target)
	if err != nil || target == nil {
		return nil, err
	}

	// A trigger's parameters are members of a scope of the transition's own, which
	// its guard and effect resolve in (symbols/bodyscopes.go).
	bodyScope := symbols.TriggerScope(scope, member)
	return &Transition{
		Name:      member.Name,
		Source:    source,
		Target:    target,
		Trigger:   classifyTrigger(member.Trigger),
		Guard:     member.Guard,
		Effect:    LowerBehaviors(member.Effect, bodyScope),
		Via:       FeaturePath(member.Via),
		Scope:     scope,
		BodyScope: bodyScope,
	}, nil
}

// isEntrySubaction reports whether member is the entry subaction of the body a
// succession was written in.
func isEntrySubaction(member ast.Node) bool {
	_, ok := unwrapMembership(member).(*ast.EntryMember)
	return ok
}

// classifyTrigger converts a raw trigger expression into a typed TriggerEvent.
// Classification rules (syntactic/structural):
// - nil → nil (completion transition)
// - *ast.TimeEvent → keep as-is (already typed from hand-built tests)
// - *ast.ChangeEvent → keep as-is
// - *ast.AcceptEvent → keep as-is
// - *ast.CallEvent → keep as-is
// - QualifiedName (bare name) → AcceptEvent{SignalType: qname} (signal trigger)
// - Expression with operators → ChangeEvent{Condition: expr} (guard-like condition)
//
// This is a syntactic heuristic - full signal vs feature disambiguation
// requires type system integration. Adequate for current SysML v2 syntax.
func classifyTrigger(trigger ast.Node) ast.Node {
	if trigger == nil {
		return nil
	}

	// Already typed events pass through
	switch trigger.(type) {
	case *ast.TimeEvent, *ast.ChangeEvent, *ast.AcceptEvent, *ast.CallEvent:
		return trigger
	}

	// A payload parameter: the accept states the occurrence it takes as a
	// parameter declaration — by its type (`accept msg : Warning`), by the event
	// it subsets (`accept :> shutDown`) — or carries a time or change event the
	// parameter was parsed with (`accept when x > 1`).
	if payload, ok := trigger.(*ast.Usage); ok {
		if payload.Value != nil {
			return classifyTrigger(payload.Value)
		}
		return &ast.AcceptEvent{
			SignalType: relationshipTarget(payload, ast.RelTyping),
			Subsets:    relationshipTarget(payload, ast.RelSubsets, ast.RelRedefines, ast.RelSpecializes, ast.RelReferences),
			Payload:    payload,
		}
	}

	// FeatureReference (bare name) → extract QualifiedName and treat as signal trigger
	if featureRef, ok := trigger.(*ast.FeatureReference); ok {
		return &ast.AcceptEvent{
			SignalType: featureRef.Name,
		}
	}

	// QualifiedName (bare name, though parser usually wraps in FeatureReference) → signal trigger
	if qname, ok := trigger.(*ast.QualifiedName); ok {
		return &ast.AcceptEvent{
			SignalType: qname,
		}
	}

	// Any other expression → change event (guard-like condition)
	return &ast.ChangeEvent{
		Condition: trigger,
	}
}

// relationshipTarget returns the qualified name a usage's first relationship of
// one of the given kinds names, or nil when it declares none.
func relationshipTarget(usage *ast.Usage, kinds ...ast.RelationshipKind) *ast.QualifiedName {
	for _, rel := range usage.Relationships {
		if rel == nil {
			continue
		}
		for _, kind := range kinds {
			if rel.Kind != kind {
				continue
			}
			if qn, ok := rel.Target.(*ast.QualifiedName); ok {
				return qn
			}
		}
	}
	return nil
}

// collectTransitions recursively processes member lists to collect transitions.
// Handles top-level members and region members.
// regionStates limits state lookup to states within a specific region (nil = all states).
// containingState is the enclosing state for sourceless transitions (nil at top level).
// scope is the scope the members were declared in.
func collectTransitions(graph *StateGraph, memberList []ast.Node, regionStates []*ast.StateNode, containingState ast.Node, scope *symbols.Scope) error {

	for _, member := range memberList {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.ErrorNode:
			// Skip error nodes
			continue
		case *ast.Usage:
			// Handle succession usages: succession statements parsed as Usage with Kind=UsageSuccession
			if n.Kind == ast.UsageSuccession {

				if len(n.ConnectorEnds) == 2 {
					// ConnectorEnds[0] is source, ConnectorEnds[1] is target
					// States could be in Target field OR Reference field

					// Try Target first, then Reference
					var sourceQName, targetQName *ast.QualifiedName

					if n.ConnectorEnds[0].Target != nil {
						sourceQName, _ = n.ConnectorEnds[0].Target.(*ast.QualifiedName)
					} else if n.ConnectorEnds[0].Reference != nil {
						sourceQName, _ = n.ConnectorEnds[0].Reference.(*ast.QualifiedName)
					}

					if n.ConnectorEnds[1].Target != nil {
						targetQName, _ = n.ConnectorEnds[1].Target.(*ast.QualifiedName)
					} else if n.ConnectorEnds[1].Reference != nil {
						targetQName, _ = n.ConnectorEnds[1].Reference.(*ast.QualifiedName)
					}

					if sourceQName != nil && targetQName != nil {

						// Use regionStates for scoped lookup if provided, otherwise all states
						searchScope := regionStates
						if searchScope == nil {
							searchScope = graph.States
						}

						sourceState := findStateByName(searchScope, sourceQName)
						targetState := findStateByName(searchScope, targetQName)

						if sourceState != nil && targetState != nil {
							trans := &Transition{
								Source:    sourceState,
								Target:    targetState,
								Trigger:   nil, // Completion transition
								Guard:     nil,
								Effect:    nil,
								Scope:     scope,
								BodyScope: scope,
							}
							graph.Transitions[sourceState] = append(graph.Transitions[sourceState], trans)
						}
					}
				}
			} else if n.Kind == ast.UsageState && len(n.Members) > 0 {
				// Handle state usages (state X { ... }) - recurse into members
				// This state usage can contain transitions (accept...then, etc.)
				// Pass this Usage node as the containing state for sourceless transitions
				if err := collectTransitions(graph, n.Members, nil, n, childScope(scope, n)); err != nil {
					return err
				}
			}
		case *ast.InitialNode:
			// Handle `initial X then Y` syntax:
			// This means "initial pseudostate transitions to state Y"
			// Mark Y as the initial state (no intermediate state for the initial node)
			if n.Successor != nil {
				searchScope := regionStates
				if searchScope == nil {
					searchScope = graph.States
				}
				targetState := findStateByName(searchScope, n.Successor)
				if targetState != nil {
					targetState.IsInitial = true
				}
			}
		case *ast.SuccessionEdge:
			// Handle succession statements: `source then target;`
			// Create completion transition from source to target
			searchScope := regionStates
			if searchScope == nil {
				searchScope = graph.States
			}

			sourceState := findStateByName(searchScope, n.Source)
			targetState := findStateByName(searchScope, n.Target)

			// `entry; then off;` — a succession out of the body's own entry
			// subaction names the state it starts in (SysML 7.19.3), the same as
			// `initial start; start then off;`.
			if sourceState == nil && targetState != nil && isEntrySubaction(n.SourceMember) {
				targetState.IsInitial = true
				continue
			}

			if sourceState != nil && targetState != nil {
				trans := &Transition{
					Source:    sourceState,
					Target:    targetState,
					Trigger:   nil, // Completion transition
					Guard:     nil,
					Effect:    nil,
					Scope:     scope,
					BodyScope: scope,
				}
				graph.Transitions[sourceState] = append(graph.Transitions[sourceState], trans)
			}
		case *ast.TransitionEdge:
			// Legacy: explicit TransitionEdge nodes (from hand-built tests)
			trans, err := lowerTransitionEdge(graph, n, scope)
			if err != nil {
				return err
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		case *ast.TransitionMember:
			// New: TransitionMember from parser (declarative)
			trans, err := lowerTransitionMember(graph, n, containingState, scope)
			if err != nil {
				return err
			}
			// An endpoint naming no vertex was reported by name resolution.
			if trans == nil {
				continue
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		case *ast.StateNode:
			// Recurse into state substates to collect transitions within the state
			// Transitions inside this state have this state as their containing state
			stateScope := graph.StateScopes[n]
			if stateScope == nil {
				stateScope = graph.stateScope(scope, n)
			}
			if err := collectTransitions(graph, n.Substates, nil, n, stateScope); err != nil {
				return err
			}
		case *ast.StateRegion:
			// The states this region declares, in collection order: a transition
			// declared inside a region names them, and collecting states already
			// recorded which region declares each one.
			var statesInRegion []*ast.StateNode
			for _, state := range graph.States {
				if graph.RegionOf[state] == n {
					statesInRegion = append(statesInRegion, state)
				}
			}

			// Recurse into region members with scoped state list
			// Regions are orthogonal - transitions within them don't inherit a containing state
			if err := collectTransitions(graph, n.States, statesInRegion, nil, childScope(scope, n)); err != nil {
				return err
			}
		}
	}
	return nil
}
