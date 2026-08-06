package lower

import (
	"fmt"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// StateGraph is the execution IR for state machines.
type StateGraph struct {
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
	Source  ast.Node   // *ast.StateNode or *ast.PseudostateNode
	Target  ast.Node   // *ast.StateNode or *ast.PseudostateNode
	Trigger ast.Node   // TimeEvent, ChangeEvent, SignalEvent, CallEvent, nil = completion
	Guard   ast.Node   // guard expression, nil = no guard
	Effect  []ast.Node // effect actions
}

// ToStateGraph converts a state machine AST (Usage or Definition) to a StateGraph.
func ToStateGraph(stateMachineDecl ast.Node) (*StateGraph, error) {
	graph := &StateGraph{
		States:           make([]*ast.StateNode, 0),
		Pseudostates:     make(map[string]*ast.PseudostateNode),
		PseudostateOwner: make(map[*ast.PseudostateNode]*ast.StateNode),
		Transitions:      make(map[ast.Node][]*Transition),
		CompositeStates:  make(map[*ast.StateNode][]*ast.StateRegion),
		RegionInitials:   make(map[*ast.StateRegion]*ast.StateNode),
		ParentState:      make(map[*ast.StateNode]*ast.StateNode),
		RegionOwner:      make(map[*ast.StateRegion]*ast.StateNode),
		RegionOf:         make(map[*ast.StateNode]*ast.StateRegion),
	}

	// Extract members
	var members []ast.Node
	switch n := stateMachineDecl.(type) {
	case *ast.Usage:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	default:
		return nil, fmt.Errorf("state machine must be Usage or Definition, got %T", stateMachineDecl)
	}

	graph.Connections = lowerConnections(members)

	// First pass: collect states and pseudostates
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.StateNode:
			collectStates(graph, n, nil)
		case *ast.Usage:
			// Handle state usages: state declarations parsed as Usage with Kind=UsageState
			if n.Kind == ast.UsageState {
				collectStates(graph, stateNodeFromUsage(n), nil)
			}
		case *ast.SubstateMember:
			// Substate declarations: state <name>;
			// Create a StateNode from the name
			stateNode := &ast.StateNode{
				Name: n.Name,
			}
			stateNode.NodeSpan = n.NodeSpan
			collectStates(graph, stateNode, nil)
		case *ast.StateRegion:
			// Top-level region: collect its states, which no state is the parent of
			collectRegionStates(graph, n, nil)
		case *ast.PseudostateNode:
			graph.Pseudostates[n.Name] = n
		}
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

	// Third pass: collect transitions
	if err := collectTransitions(graph, members, nil, nil); err != nil {
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
// them; without this the body's behaviors are silently dropped.
func stateNodeFromUsage(usage *ast.Usage) *ast.StateNode {
	state := &ast.StateNode{Name: usage.Ident.Name}
	state.NodeSpan = usage.NodeSpan

	for _, member := range usage.Members {
		switch m := unwrapMembership(member).(type) {
		case *ast.EntryMember:
			state.Entry = append(state.Entry, m.Actions...)
		case *ast.DoMember:
			state.Do = append(state.Do, m.Actions...)
		case *ast.ExitMember:
			state.Exit = append(state.Exit, m.Actions...)
		case *ast.StateRegion:
			state.Regions = append(state.Regions, m)
		case *ast.StateNode:
			state.Substates = append(state.Substates, m)
		case *ast.PseudostateNode:
			state.Substates = append(state.Substates, m)
		case *ast.SubstateMember:
			child := &ast.StateNode{Name: m.Name}
			child.NodeSpan = m.NodeSpan
			state.Substates = append(state.Substates, child)
		case *ast.Usage:
			// A state declared inside a composite state is one of its substates:
			// dropping it here would leave the hierarchy out of the graph and every
			// transition naming a nested state unresolvable.
			if m.Kind == ast.UsageState {
				state.Substates = append(state.Substates, stateNodeFromUsage(m))
			}
		}
	}
	return state
}

// collectStates recursively collects states and builds parent relationships.
func collectStates(graph *StateGraph, state *ast.StateNode, parent *ast.StateNode) {
	graph.States = append(graph.States, state)
	if parent != nil {
		graph.ParentState[state] = parent
	}

	// Recursively collect substates
	for _, substate := range state.Substates {
		switch child := unwrapMembership(substate).(type) {
		case *ast.StateNode:
			collectStates(graph, child, state)
		case *ast.PseudostateNode:
			// A pseudostate declared inside a composite state belongs to it: that
			// ownership is what a history pseudostate restores from, and without it
			// a nested pseudostate is not part of the graph at all.
			graph.Pseudostates[child.Name] = child
			graph.PseudostateOwner[child] = state
		}
	}

	// Collect states in orthogonal regions
	for _, region := range state.Regions {
		collectRegionStates(graph, region, state)
	}
}

// collectRegionStates collects the states an orthogonal region declares, records
// which region declares each of them and which one the region starts in, and
// assigns the region's pseudostates to the state that owns the region. parent is
// the state owning the region, nil for the machine's own regions.
//
// Region members reach here as a state node, a bare `state <name>;` substate or a
// state usage with a body, each of them possibly wrapped in a membership: a state
// missed here is a state no transition can name.
func collectRegionStates(graph *StateGraph, region *ast.StateRegion, parent *ast.StateNode) {
	for _, member := range region.States {
		var state *ast.StateNode
		switch n := unwrapMembership(member).(type) {
		case *ast.StateNode:
			state = n
		case *ast.SubstateMember:
			state = &ast.StateNode{Name: n.Name}
			state.NodeSpan = n.NodeSpan
		case *ast.Usage:
			if n.Kind != ast.UsageState {
				continue
			}
			state = stateNodeFromUsage(n)
		case *ast.PseudostateNode:
			graph.Pseudostates[n.Name] = n
			if parent != nil {
				graph.PseudostateOwner[n] = parent
			}
			continue
		default:
			continue
		}

		collectStates(graph, state, parent)
		graph.RegionOf[state] = region
		if state.IsInitial && graph.RegionInitials[region] == nil {
			graph.RegionInitials[region] = state
		}
	}
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

// lowerTransitionEdge converts a TransitionEdge (legacy) to a Transition.
func lowerTransitionEdge(graph *StateGraph, edge *ast.TransitionEdge) (*Transition, error) {
	sourceState := findStateByName(graph.States, edge.Source)
	if sourceState == nil {
		return nil, fmt.Errorf("transition edge references undefined source state %v", edge.Source)
	}

	targetState := findStateByName(graph.States, edge.Target)
	if targetState == nil {
		return nil, fmt.Errorf("transition edge references undefined target state %v", edge.Target)
	}

	return &Transition{
		Source:  sourceState,
		Target:  targetState,
		Trigger: edge.Trigger,
		Guard:   edge.Guard,
		Effect:  edge.Effect,
	}, nil
}

// lowerTransitionMember converts a TransitionMember (parser output) to a Transition.
// containingState is used as the source when member.Source is nil (sourceless accept...then).
func lowerTransitionMember(graph *StateGraph, member *ast.TransitionMember, containingState ast.Node) (*Transition, error) {
	// TransitionMember has: Source (QualifiedName), Target (QualifiedName), Trigger, Guard, Effect ([]Node)
	// Source and Target can be StateNode or PseudostateNode
	// Source can be nil for sourceless transitions (accept...then) - use containingState

	// Try to find source as state
	var source ast.Node
	if member.Source == nil {
		// Sourceless transition - use containing state
		if containingState == nil {
			return nil, fmt.Errorf("sourceless transition (accept...then) at top level has no containing state")
		}

		// containingState could be *ast.Usage (for state X { ... }) or *ast.StateNode
		// Need to find the corresponding StateNode in graph.States
		switch cs := containingState.(type) {
		case *ast.StateNode:
			source = cs
		case *ast.Usage:
			// Find the StateNode that corresponds to this Usage
			// Match by checking if the Usage is the source of any state
			for _, s := range graph.States {
				// StateNode typically comes from the same parse tree - check identity
				// Or match by name if available
				if s.Name == cs.Ident.Name {
					source = s
					break
				}
			}
			if source == nil {
				return nil, fmt.Errorf("could not resolve containing state Usage %q to StateNode", cs.Ident.Name)
			}
		default:
			return nil, fmt.Errorf("containing state has unexpected type %T", containingState)
		}
	} else {
		sourceState := findStateByName(graph.States, member.Source)
		if sourceState != nil {
			source = sourceState
		} else {
			// Try pseudostate
			sourceName := member.Source.Parts[len(member.Source.Parts)-1].Text
			if ps, ok := graph.Pseudostates[sourceName]; ok {
				source = ps
			}
		}

		if source == nil {
			// Debug: print available states and pseudostates
			var stateNames []string
			for _, s := range graph.States {
				stateNames = append(stateNames, s.Name)
			}
			for name := range graph.Pseudostates {
				stateNames = append(stateNames, name+" (pseudostate)")
			}
			return nil, fmt.Errorf("transition member references undefined source %q (available: %v)",
				member.Source.Parts[len(member.Source.Parts)-1].Text, stateNames)
		}
	}

	// Try to find target as state or pseudostate
	var target ast.Node
	targetState := findStateByName(graph.States, member.Target)
	if targetState != nil {
		target = targetState
	} else {
		// Try pseudostate
		targetName := member.Target.Parts[len(member.Target.Parts)-1].Text
		if ps, ok := graph.Pseudostates[targetName]; ok {
			target = ps
		}
	}

	if target == nil {
		var stateNames []string
		for _, s := range graph.States {
			stateNames = append(stateNames, s.Name)
		}
		for name := range graph.Pseudostates {
			stateNames = append(stateNames, name+" (pseudostate)")
		}
		return nil, fmt.Errorf("transition member references undefined target %q (available: %v)",
			member.Target.Parts[len(member.Target.Parts)-1].Text, stateNames)
	}

	return &Transition{
		Source:  source,
		Target:  target,
		Trigger: classifyTrigger(member.Trigger),
		Guard:   member.Guard,
		Effect:  member.Effect,
	}, nil
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

// collectTransitions recursively processes member lists to collect transitions.
// Handles top-level members and region members.
// regionStates limits state lookup to states within a specific region (nil = all states).
// containingState is the enclosing state for sourceless transitions (nil at top level).
func collectTransitions(graph *StateGraph, memberList []ast.Node, regionStates []*ast.StateNode, containingState ast.Node) error {

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
								Source:  sourceState,
								Target:  targetState,
								Trigger: nil, // Completion transition
								Guard:   nil,
								Effect:  nil,
							}
							graph.Transitions[sourceState] = append(graph.Transitions[sourceState], trans)
						}
					}
				}
			} else if n.Kind == ast.UsageState && len(n.Members) > 0 {
				// Handle state usages (state X { ... }) - recurse into members
				// This state usage can contain transitions (accept...then, etc.)
				// Pass this Usage node as the containing state for sourceless transitions
				if err := collectTransitions(graph, n.Members, nil, n); err != nil {
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

			if sourceState != nil && targetState != nil {
				trans := &Transition{
					Source:  sourceState,
					Target:  targetState,
					Trigger: nil, // Completion transition
					Guard:   nil,
					Effect:  nil,
				}
				graph.Transitions[sourceState] = append(graph.Transitions[sourceState], trans)
			}
		case *ast.TransitionEdge:
			// Legacy: explicit TransitionEdge nodes (from hand-built tests)
			trans, err := lowerTransitionEdge(graph, n)
			if err != nil {
				return err
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		case *ast.TransitionMember:
			// New: TransitionMember from parser (declarative)
			trans, err := lowerTransitionMember(graph, n, containingState)
			if err != nil {
				return err
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		case *ast.StateNode:
			// Recurse into state substates to collect transitions within the state
			// Transitions inside this state have this state as their containing state
			if err := collectTransitions(graph, n.Substates, nil, n); err != nil {
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
			if err := collectTransitions(graph, n.States, statesInRegion, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
