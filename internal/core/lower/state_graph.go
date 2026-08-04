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
	
	// Transitions: source state → list of transitions
	Transitions map[*ast.StateNode][]*Transition
	
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
	Source  *ast.StateNode
	Target  *ast.StateNode
	Trigger ast.Node           // TimeEvent, ChangeEvent, SignalEvent, CallEvent, nil = completion
	Guard   ast.Node           // guard expression, nil = no guard
	Effect  []ast.Node         // effect actions
}

// ToStateGraph converts a state machine AST (Usage or Definition) to a StateGraph.
func ToStateGraph(stateMachineDecl ast.Node) (*StateGraph, error) {
	graph := &StateGraph{
		States:          make([]*ast.StateNode, 0),
		Pseudostates:    make(map[string]*ast.PseudostateNode),
		Transitions:     make(map[*ast.StateNode][]*Transition),
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
		
		if state, ok := actualMember.(*ast.StateNode); ok {
			collectStates(graph, state, nil)
		} else if ps, ok := actualMember.(*ast.PseudostateNode); ok {
			graph.Pseudostates[ps.Name] = ps
		}
	}
	
	// Second pass: identify composite states with regions
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
	
	// Find initial state (for simple machines)
	for _, state := range graph.States {
		if state.IsInitial && graph.ParentState[state] == nil {
			if graph.Initial != nil {
				return nil, fmt.Errorf("state machine has multiple top-level initial states")
			}
			graph.Initial = state
		}
	}
	
	// Validate: must have initial state OR top-level regions
	hasTopLevelRegions := false
	for _, state := range graph.States {
		if graph.ParentState[state] == nil && len(state.Regions) > 0 {
			hasTopLevelRegions = true
			break
		}
	}
	
	if graph.Initial == nil && !hasTopLevelRegions {
		return nil, fmt.Errorf("state machine has no initial state")
	}
	
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
	// TransitionMember has: Source (QualifiedName), Target (QualifiedName), Trigger, Guard, Effect ([]Node)
	sourceState := findStateByName(graph.States, member.Source)
	if sourceState == nil {
		return nil, fmt.Errorf("transition member references undefined source state %v", member.Source)
	}
	
	targetState := findStateByName(graph.States, member.Target)
	if targetState == nil {
		return nil, fmt.Errorf("transition member references undefined target state %v", member.Target)
	}
	
	return &Transition{
		Source:  sourceState,
		Target:  targetState,
		Trigger: member.Trigger,
		Guard:   member.Guard,
		Effect:  member.Effect,
	}, nil
}
