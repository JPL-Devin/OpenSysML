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
	
	// InitialState (required for simple machines, nil for multi-region)
	Initial *ast.StateNode
}

// Transition represents a state transition (lowered from TransitionEdge or TransitionMember).
type Transition struct {
	Source  ast.Node           // *ast.StateNode or *ast.PseudostateNode
	Target  ast.Node           // *ast.StateNode or *ast.PseudostateNode
	Trigger ast.Node           // TimeEvent, ChangeEvent, SignalEvent, CallEvent, nil = completion
	Guard   ast.Node           // guard expression, nil = no guard
	Effect  []ast.Node         // effect actions
}

// ToStateGraph converts a state machine AST (Usage or Definition) to a StateGraph.
func ToStateGraph(stateMachineDecl ast.Node) (*StateGraph, error) {
	graph := &StateGraph{
		States:          make([]*ast.StateNode, 0),
		Pseudostates:    make(map[string]*ast.PseudostateNode),
		Transitions:     make(map[ast.Node][]*Transition),
		CompositeStates: make(map[*ast.StateNode][]*ast.StateRegion),
		RegionInitials:  make(map[*ast.StateRegion]*ast.StateNode),
		ParentState:     make(map[*ast.StateNode]*ast.StateNode),
		RegionOwner:     make(map[*ast.StateRegion]*ast.StateNode),
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
	
	// First pass: collect states and pseudostates
	for _, member := range members {
		actualMember := unwrapMembership(member)
		
		switch n := actualMember.(type) {
		case *ast.StateNode:
			collectStates(graph, n, nil)
		case *ast.SubstateMember:
			// Substate declarations: state <name>;
			// Create a StateNode from the name
			stateNode := &ast.StateNode{
				Name: n.Name,
			}
			stateNode.NodeSpan = n.NodeSpan
			collectStates(graph, stateNode, nil)
		case *ast.StateRegion:
			// Top-level region: collect states within region
			for _, regionMember := range n.States {
				if state, ok := regionMember.(*ast.StateNode); ok {
					collectStates(graph, state, nil)
				} else if substate, ok := regionMember.(*ast.SubstateMember); ok {
					stateNode := &ast.StateNode{Name: substate.Name}
					stateNode.NodeSpan = substate.NodeSpan
					collectStates(graph, stateNode, nil)
				}
			}
			// Store the region for later processing
			// (We'll handle RegionInitials after collecting all states)
		case *ast.PseudostateNode:
			graph.Pseudostates[n.Name] = n
		}
	}
	
	// Second pass: identify composite states with regions AND handle top-level regions
	// Check for top-level regions (state machine itself has regions as members)
	for _, member := range members {
		actualMember := unwrapMembership(member)
		if region, ok := actualMember.(*ast.StateRegion); ok {
			// Top-level region
			initialState := findInitialStateInRegion(graph, region)
			if initialState == nil {
				return nil, fmt.Errorf("top-level region %s has no initial state", region.Name)
			}
			graph.RegionInitials[region] = initialState
		}
	}
	
	// Also handle states that have regions as sub-members
	for _, state := range graph.States {
		if len(state.Regions) > 0 {
			graph.CompositeStates[state] = state.Regions
			
			for _, region := range state.Regions {
				graph.RegionOwner[region] = state
				
				initialState := findInitialStateInRegion(graph, region)
				if initialState == nil {
					return nil, fmt.Errorf("region %s in state %s has no initial state", region.Name, state.Name)
				}
				graph.RegionInitials[region] = initialState
			}
		}
	}
	
	// Third pass: collect transitions
	for _, member := range members {
		actualMember := unwrapMembership(member)
		
		switch n := actualMember.(type) {
		case *ast.ErrorNode:
			// Skip error nodes
			continue
		case *ast.InitialNode:
			// Handle `first X then Y` syntax
			if n.Successor != nil {
				targetState := findStateByName(graph.States, n.Successor)
				if targetState != nil {
					targetState.IsInitial = true
				}
			}
		case *ast.TransitionEdge:
			// Legacy: explicit TransitionEdge nodes (from hand-built tests)
			trans, err := lowerTransitionEdge(graph, n)
			if err != nil {
				return nil, err
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		case *ast.TransitionMember:
			// New: TransitionMember from parser (declarative)
			trans, err := lowerTransitionMember(graph, n)
			if err != nil {
				return nil, err
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		}
	}
	
	// Find initial state (for simple machines - machines without top-level regions)
	if len(graph.RegionInitials) == 0 {
		// No top-level regions, find the simple initial state
		for _, state := range graph.States {
			if state.IsInitial && graph.ParentState[state] == nil {
				if graph.Initial != nil {
					return nil, fmt.Errorf("state machine has multiple top-level initial states")
				}
				graph.Initial = state
			}
		}
	}
	// If there are top-level regions, Initial stays nil (executor will use RegionInitials instead)
	
	// Note: Initial state is optional at graph construction time.
	// The executor's initialize() will validate and return the error if missing.
	// Top-level regions are also valid (no single initial state).
	
	return graph, nil
}

// collectStates recursively collects states and builds parent relationships.
func collectStates(graph *StateGraph, state *ast.StateNode, parent *ast.StateNode) {
	graph.States = append(graph.States, state)
	if parent != nil {
		graph.ParentState[state] = parent
	}
	
	// Recursively collect substates
	for _, substate := range state.Substates {
		if childState, ok := substate.(*ast.StateNode); ok {
			collectStates(graph, childState, state)
		}
	}
	
	// Collect states in orthogonal regions
	for _, region := range state.Regions {
		for _, regionState := range region.States {
			if childState, ok := regionState.(*ast.StateNode); ok {
				collectStates(graph, childState, state)
			}
		}
	}
}

// findInitialStateInRegion finds the initial state in a region.
func findInitialStateInRegion(graph *StateGraph, region *ast.StateRegion) *ast.StateNode {
	for _, state := range region.States {
		if stateNode, ok := state.(*ast.StateNode); ok {
			if stateNode.IsInitial {
				return stateNode
			}
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
		Effect:  nil, // TransitionEdge has no effect field
	}, nil
}

// lowerTransitionMember converts a TransitionMember (parser output) to a Transition.
func lowerTransitionMember(graph *StateGraph, member *ast.TransitionMember) (*Transition, error) {
	// Debug
	fmt.Printf("DEBUG lowerTransitionMember: Source=%v, Target=%v\n", member.Source, member.Target)
	
	// TransitionMember has: Source (QualifiedName), Target (QualifiedName), Trigger, Guard, Effect ([]Node)
	// Source and Target can be StateNode or PseudostateNode
	
	// Try to find source as state
	var source ast.Node
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
		Trigger: member.Trigger,
		Guard:   member.Guard,
		Effect:  member.Effect,
	}, nil
}
