package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// StateConfiguration represents the active state configuration (simple or multi-region).
type StateConfiguration struct {
	// For simple states (no regions): single active state
	simpleState *ast.StateNode
	
	// For composite states with regions: map of region → active state in that region
	regionStates map[*ast.StateRegion]*ast.StateNode
	
	// Hierarchical parent chain (for history and LCA calculations)
	hierarchyStack []*ast.StateNode
}

// StateExecutor executes state machines using event-driven semantics.
type StateExecutor struct {
	ctx          *Context
	stateMachine *symbols.Symbol
	state        ExecutionState
	trace        *TraceRecorder // Optional trace recorder for testing
	
	// Lowered graph (source of truth)
	graph *lower.StateGraph
	
	// State machine execution state
	activeConfig *StateConfiguration  // Active state configuration (simple or multi-region)
	currentTime  float64
	eventQueue   *EventQueue
	stateData    map[string]Value // State machine local variables
	
	// Graph structure (populated from graph for backward compatibility)
	states         []*ast.StateNode
	pseudostates   map[string]*ast.PseudostateNode          // Pseudostates by name
	transitions    map[*ast.StateNode][]*ast.TransitionEdge
	compositeStates map[*ast.StateNode][]*ast.StateRegion  // States with orthogonal regions
	regionInitials map[*ast.StateRegion]*ast.StateNode     // Initial state per region
	
	// Hierarchical state support
	stateStack  []*ast.StateNode                        // Active state configuration (for nested states)
	parentState map[*ast.StateNode]*ast.StateNode       // Child -> parent mapping
	regionOwner map[*ast.StateRegion]*ast.StateNode    // Region -> parent state
}

// newStateExecutor creates a state executor.
func newStateExecutor(ctx *Context, stateMachine *symbols.Symbol) (*StateExecutor, error) {
	if stateMachine.Kind != symbols.SymbolStateUsage && stateMachine.Kind != symbols.SymbolStateDef {
		return nil, fmt.Errorf("symbol %s is not a state machine", stateMachine.Name)
	}
	
	// Lower to StateGraph
	graph, err := lower.ToStateGraph(stateMachine.Decl)
	if err != nil {
		return nil, fmt.Errorf("lower state machine: %w", err)
	}
	
	exec := &StateExecutor{
		ctx:             ctx,
		stateMachine:    stateMachine,
		state:           StateReady,
		graph:           graph,
		currentTime:     0.0,
		eventQueue:      NewEventQueue(),
		stateData:       make(map[string]Value),
		pseudostates:    make(map[string]*ast.PseudostateNode),
		transitions:     make(map[*ast.StateNode][]*ast.TransitionEdge),
		compositeStates: make(map[*ast.StateNode][]*ast.StateRegion),
		regionInitials:  make(map[*ast.StateRegion]*ast.StateNode),
		stateStack:      make([]*ast.StateNode, 0),
		parentState:     make(map[*ast.StateNode]*ast.StateNode),
		regionOwner:     make(map[*ast.StateRegion]*ast.StateNode),
		activeConfig: &StateConfiguration{
			regionStates: make(map[*ast.StateRegion]*ast.StateNode),
		},
	}
	
	// Populate internal structures from graph for backward compatibility
	if err := exec.populateFromGraph(); err != nil {
		return nil, fmt.Errorf("populate from graph: %w", err)
	}
	
	return exec, nil
}

// populateFromGraph populates internal structures from the lowered StateGraph.
// This is a temporary bridge - eventually all code should use e.graph directly.
func (e *StateExecutor) populateFromGraph() error {
	// Copy states
	e.states = e.graph.States
	
	// Copy pseudostates
	for name, ps := range e.graph.Pseudostates {
		e.pseudostates[name] = ps
	}
	
	// Copy composite states and regions
	for state, regions := range e.graph.CompositeStates {
		e.compositeStates[state] = regions
		
		// Track region ownership
		for _, region := range regions {
			e.regionOwner[region] = state
			
			// Find initial state in this region
			if initialState, ok := e.graph.RegionInitials[region]; ok {
				e.regionInitials[region] = initialState
			}
		}
	}
	
	// Copy parent state mappings
	for child, parent := range e.graph.ParentState {
		e.parentState[child] = parent
	}
	
	// Convert lower.Transition back to TransitionEdge for backward compatibility
	// TODO: Eventually migrate all transition-handling code to use lower.Transition
	for sourceNode, transList := range e.graph.Transitions {
		// Type assert source to StateNode (skip pseudostates for now - they're handled separately)
		sourceState, ok := sourceNode.(*ast.StateNode)
		if !ok {
			continue // Skip transitions from pseudostates (handled by evaluateChoicePseudostate etc.)
		}
		
		for _, lowerTrans := range transList {
			// Get source/target names (handle both StateNode and PseudostateNode)
			sourceName := getNodeName(lowerTrans.Source)
			targetName := getNodeName(lowerTrans.Target)
			
			// Create TransitionEdge for backward compatibility
			edge := &ast.TransitionEdge{
				Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: sourceName}}},
				Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: targetName}}},
				Guard:   lowerTrans.Guard,
				Effect:  lowerTrans.Effect,
			}
			
			// Type assert Trigger to TriggerEvent (TimeEvent, ChangeEvent, etc. implement this interface)
			if lowerTrans.Trigger != nil {
				if triggerEvent, ok := lowerTrans.Trigger.(ast.TriggerEvent); ok {
					edge.Trigger = triggerEvent
				}
			}
			
			e.transitions[sourceState] = append(e.transitions[sourceState], edge)
		}
	}
	
	return nil
}

// getNodeName returns the name of a StateNode or PseudostateNode.
func getNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.StateNode:
		return n.Name
	case *ast.PseudostateNode:
		return n.Name
	default:
		return ""
	}
}

// getParentChain returns all ancestor states from child to root (inclusive).
// Result is ordered: [child, parent, grandparent, ...]
func (e *StateExecutor) getParentChain(state *ast.StateNode) []*ast.StateNode {
	chain := []*ast.StateNode{state}
	current := state
	for {
		parent, hasParent := e.parentState[current]
		if !hasParent {
			break
		}
		chain = append(chain, parent)
		current = parent
	}
	return chain
}

// getLCA finds the lowest common ancestor of two states.
// Returns nil if states are in different hierarchies.
func (e *StateExecutor) getLCA(state1, state2 *ast.StateNode) *ast.StateNode {
	chain1 := e.getParentChain(state1)
	chain2 := e.getParentChain(state2)
	
	// Build set from chain1
	chain1Set := make(map[*ast.StateNode]bool)
	for _, s := range chain1 {
		chain1Set[s] = true
	}
	
	// Find first common ancestor in chain2
	for _, s := range chain2 {
		if chain1Set[s] {
			return s
		}
	}
	
	return nil // No common ancestor
}

// findStateByName looks up a state by qualified name.
func (e *StateExecutor) findStateByName(qname *ast.QualifiedName) *ast.StateNode {
	if qname == nil || len(qname.Parts) == 0 {
		return nil
	}
	
	targetName := qname.Parts[len(qname.Parts)-1].Text
	for _, state := range e.states {
		if state.Name == targetName {
			return state
		}
	}
	
	return nil
}

// findInitialStateInRegion finds the initial state within a region.
func (e *StateExecutor) findInitialStateInRegion(region *ast.StateRegion) *ast.StateNode {
	for _, member := range region.States {
		if state, ok := member.(*ast.StateNode); ok {
			if state.IsInitial {
				return state
			}
		}
	}
	return nil
}

// scheduleTransitionEvents schedules TimeEvents for outgoing transitions from current state.
func (e *StateExecutor) scheduleTransitionEvents() error {
	currentState := e.getCurrentState()
	if currentState == nil {
		return nil // Multi-region state, skip for now (would need per-region scheduling)
	}
	transitions := e.transitions[currentState]
	
	for _, trans := range transitions {
		if timeEvent, ok := trans.Trigger.(*ast.TimeEvent); ok {
			// Evaluate duration expression
			ec := NewEvalContext(e.ctx, nil)
			ec.Push(e.stateData)
			durationVal, err := ec.Eval(timeEvent.Duration)
			if err != nil {
				return fmt.Errorf("eval time duration: %w", err)
			}
			
			// Extract numeric duration
			var duration float64
			if durationVal.Kind == ValConst {
				switch durationVal.Const.Kind {
				case semantics.ValInt:
					duration = float64(durationVal.Const.Int)
				case semantics.ValReal:
					duration = durationVal.Const.Real
				default:
					return fmt.Errorf("time duration must be numeric, got %v", durationVal.Const.Kind)
				}
			} else {
				return fmt.Errorf("time duration must be constant, got %v", durationVal.Kind)
			}
			
			// Schedule event (generate unique ID using current queue length)
			e.eventQueue.Push(Event{
				ID:        int64(e.eventQueue.Len() + 1),
				Type:      EventTime,
				Timestamp: e.currentTime + duration,
				Payload:   trans, // Store transition reference
			})
		}
	}
	
	return nil
}

// processNextEvent pops and processes the next event from queue.
func (e *StateExecutor) processNextEvent() error {
	if e.eventQueue.Len() == 0 {
		return fmt.Errorf("no events to process")
	}
	
	event := e.eventQueue.Pop()
	
	// Advance time
	e.currentTime = event.Timestamp
	
	// Process event by type
	switch event.Type {
	case EventTime:
		// Fire transition
		transition, ok := event.Payload.(*ast.TransitionEdge)
		if !ok {
			return fmt.Errorf("invalid TimeEvent payload")
		}
		return e.fireTransition(transition)
	default:
		// For general events, broadcast to all active regions
		return e.broadcastEvent(&event)
	}
}

// broadcastEvent sends an event to all active regions (or single state if no regions).
func (e *StateExecutor) broadcastEvent(event *Event) error {
	// If in composite state with regions, broadcast to all
	if len(e.activeConfig.regionStates) > 0 {
		// Broadcast event to each region independently
		for region, regionState := range e.activeConfig.regionStates {
			// Find transitions from this region's active state
			transitions := e.transitions[regionState]
			for _, trans := range transitions {
				if e.matchesEvent(trans, event) {
					// Fire transition within this region
					if err := e.fireTransitionInRegion(region, trans); err != nil {
						return fmt.Errorf("fire transition in region %s: %w", region.Name, err)
					}
					break // Run-to-completion: one transition per region per event
				}
			}
		}
		return nil
	}
	
	// Simple state (no regions) - existing logic
	currentState := e.getCurrentState()
	if currentState == nil {
		return nil // No active state
	}
	
	transitions := e.transitions[currentState]
	for _, trans := range transitions {
		if e.matchesEvent(trans, event) {
			return e.fireTransition(trans)
		}
	}
	
	return nil // Event not consumed
}

// matchesEvent checks if a transition matches the given event.
func (e *StateExecutor) matchesEvent(trans *ast.TransitionEdge, event *Event) bool {
	// For now, simplified: no explicit event matching
	// In full UML, would match SignalEvent, ChangeEvent, etc.
	return true
}

// fireTransitionInRegion fires a transition within a specific region.
func (e *StateExecutor) fireTransitionInRegion(region *ast.StateRegion, trans *ast.TransitionEdge) error {
	// Find target state
	targetState := e.findStateByName(trans.Target)
	if targetState == nil {
		return fmt.Errorf("transition target state not found")
	}
	
	// Get current state in this region
	sourceState := e.activeConfig.regionStates[region]
	
	// Evaluate guard if present
	if trans.Guard != nil {
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData)
		guardVal, err := ec.Eval(trans.Guard)
		if err != nil {
			return fmt.Errorf("eval guard: %w", err)
		}
		
		guardPass := false
		if guardVal.Kind == ValConst && guardVal.Const.Kind == semantics.ValBool {
			guardPass = guardVal.Const.Bool
		} else {
			return fmt.Errorf("guard must be boolean, got %v", guardVal.Kind)
		}
		
		if !guardPass {
			return nil // Guard failed, remain in current state
		}
	}
	
	// Exit current state in this region
	if err := e.exitState(sourceState); err != nil {
		return fmt.Errorf("exit state: %w", err)
	}
	
	// Execute transition effect
	for _, action := range trans.Effect {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}
	
	// Enter target state in this region
	if err := e.enterState(targetState); err != nil {
		return fmt.Errorf("enter state: %w", err)
	}
	
	// Update active state for this region
	e.activeConfig.regionStates[region] = targetState
	
	// Record trace
	if e.trace != nil {
		eventName := ""
		if trans.Trigger != nil {
			eventName = fmt.Sprintf("%v", trans.Trigger)
		}
		e.trace.RecordStateTransition(sourceState.Name, targetState.Name, eventName)
	}
	
	return nil
}

// fireTransition executes a state transition.
func (e *StateExecutor) fireTransition(trans *ast.TransitionEdge) error {
	// Check if target is a pseudostate (choice or junction)
	var targetState *ast.StateNode
	if trans.Target != nil && len(trans.Target.Parts) > 0 {
		targetName := trans.Target.Parts[len(trans.Target.Parts)-1].Text
		
		// Check if it's a pseudostate first
		if ps, exists := e.pseudostates[targetName]; exists {
			var err error
			switch ps.Kind {
			case ast.PseudostateChoice:
				targetState, err = e.evaluateChoicePseudostate(ps)
			case ast.PseudostateJunction:
				targetState, err = e.evaluateJunctionPseudostate(ps)
			default:
				return fmt.Errorf("unsupported pseudostate kind: %v", ps.Kind)
			}
			
			if err != nil {
				return fmt.Errorf("evaluate pseudostate: %w", err)
			}
		} else {
			// Normal state target
			targetState = e.findStateByName(trans.Target)
		}
	}
	
	if targetState == nil {
		return fmt.Errorf("transition target state not found")
	}
	
	// Evaluate guard if present
	if trans.Guard != nil {
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData)
		guardVal, err := ec.Eval(trans.Guard)
		if err != nil {
			return fmt.Errorf("eval guard: %w", err)
		}
		
		// Check if boolean true
		guardPass := false
		if guardVal.Kind == ValConst && guardVal.Const.Kind == semantics.ValBool {
			guardPass = guardVal.Const.Bool
		} else {
			return fmt.Errorf("guard must be boolean, got %v", guardVal.Kind)
		}
		
		// Block transition if guard false
		if !guardPass {
			return nil // Remain in current state
		}
	}
	
	// Exit current state hierarchy up to LCA
	currentState := e.getCurrentState()
	lca := e.getLCA(currentState, targetState)
	statesToExit := make([]*ast.StateNode, 0)
	current := currentState
	for current != nil && current != lca {
		statesToExit = append(statesToExit, current)
		current = e.parentState[current]
	}
	
	// Exit states (deepest to shallowest)
	for _, state := range statesToExit {
		if err := e.exitState(state); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	
	// Execute transition effect
	for _, action := range trans.Effect {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}
	
	// Enter target state hierarchy from LCA
	statesToEnter := make([]*ast.StateNode, 0)
	current = targetState
	for current != nil && current != lca {
		statesToEnter = append(statesToEnter, current)
		current = e.parentState[current]
	}
	
	// Reverse statesToEnter (shallowest to deepest)
	for i := len(statesToEnter) - 1; i >= 0; i-- {
		if err := e.enterState(statesToEnter[i]); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
	}
	
	// Update current state and rebuild stateStack with full active configuration
	e.setCurrentState(targetState)
	e.stateStack = e.getParentChain(targetState)
	// Reverse to root→leaf order for stateStack
	for i, j := 0, len(e.stateStack)-1; i < j; i, j = i+1, j-1 {
		e.stateStack[i], e.stateStack[j] = e.stateStack[j], e.stateStack[i]
	}
	
	// Schedule new events
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}
	
	// Check if final state
	if targetState.IsFinal {
		e.state = StateCompleted
	}
	
	// Record trace
	if e.trace != nil {
		fromName := ""
		currentState := e.getCurrentState()
		if currentState != nil {
			fromName = currentState.Name
		}
		eventName := ""
		if trans.Trigger != nil {
			// Extract event name from trigger (simplified)
			eventName = fmt.Sprintf("%v", trans.Trigger)
		}
		e.trace.RecordStateTransition(fromName, targetState.Name, eventName)
	}
	
	return nil
}
// initialize sets current state to initial state and enters it.
func (e *StateExecutor) initialize() error {
	// Use initial state from graph
	if e.graph.Initial != nil {
		// Simple state machine with single initial state
		initialState := e.graph.Initial
		
		// Enter initial state hierarchy (parent to child)
		e.setCurrentState(initialState)
		e.state = StateRunning
		
		// Build stateStack with full active configuration (root → leaf)
		chain := e.getParentChain(initialState)
		// Reverse to root→leaf order
		for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
			chain[i], chain[j] = chain[j], chain[i]
		}
		e.stateStack = chain
		
		// Enter states from root to initial state
		for _, state := range e.stateStack {
			if err := e.enterState(state); err != nil {
				return fmt.Errorf("enter state %s: %w", state.Name, err)
			}
		}
		
		// Schedule events for outgoing transitions
		if err := e.scheduleTransitionEvents(); err != nil {
			return fmt.Errorf("schedule events: %w", err)
		}
		
		return nil
	}
	
	// State machine has orthogonal regions at top level
	// Find regions from composite states map (graph has already extracted them)
	if len(e.graph.RegionInitials) == 0 {
		return fmt.Errorf("no initial state found in state machine %s", e.stateMachine.Name)
	}
	
	e.state = StateRunning
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
	e.activeConfig.simpleState = nil
	
	// Enter initial states for all regions
	for region, initialState := range e.graph.RegionInitials {
		if initialState == nil {
			return fmt.Errorf("region %s has no initial state", region.Name)
		}
		e.activeConfig.regionStates[region] = initialState
		if err := e.enterState(initialState); err != nil {
			return fmt.Errorf("enter initial state in region %s: %w", region.Name, err)
		}
	}
	
	return nil
}

// findDeepestInitialState finds initial state, following nested initial states.
func (e *StateExecutor) findDeepestInitialState() *ast.StateNode {
	// Find top-level initial state (no parent or parent not initial)
	var current *ast.StateNode
	for _, state := range e.states {
		if !state.IsInitial {
			continue
		}
		// Check if parent exists and is initial - skip if so (we want root initial)
		parent := e.parentState[state]
		if parent == nil || !parent.IsInitial {
			current = state
			break
		}
	}
	
	if current == nil {
		return nil
	}
	
	// Follow nested initial states down to deepest level
	for {
		foundNested := false
		for _, state := range e.states {
			if state.IsInitial && e.parentState[state] == current {
				current = state
				foundNested = true
				break
			}
		}
		if !foundNested {
			break
		}
	}
	
	return current
}

// enterState executes entry behaviors when entering a state.
func (e *StateExecutor) enterState(state *ast.StateNode) error {
	if state == nil {
		return nil
	}
	
	// Record trace
	if e.trace != nil {
		e.trace.RecordStateEntry(state.Name, len(state.Entry) > 0)
	}
	
	// Execute entry actions
	for _, action := range state.Entry {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("entry action: %w", err)
		}
	}
	
	// Check if this is a composite state with regions
	if regions, isComposite := e.compositeStates[state]; isComposite {
		// Entering composite state with orthogonal regions
		// Initialize active configuration for all regions
		e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
		e.activeConfig.simpleState = nil // Clear simple state
		
		for _, region := range regions {
			initialState := e.regionInitials[region]
			e.activeConfig.regionStates[region] = initialState
			// Recursively enter initial state in each region
			if err := e.enterState(initialState); err != nil {
				return fmt.Errorf("enter initial state in region %s: %w", region.Name, err)
			}
		}
	} else {
		// Simple state (no regions)
		e.activeConfig.simpleState = state
		e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode) // Clear regions
	}
	
	// Execute do activities (ongoing behavior)
	// Simplified: execute immediately like entry actions
	// Full UML semantics: concurrent execution with state lifetime
	for _, action := range state.Do {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("do action: %w", err)
		}
	}
	
	return nil
}

// exitState executes exit behaviors when leaving a state.
func (e *StateExecutor) exitState(state *ast.StateNode) error {
	if state == nil {
		return nil
	}
	
	// If this is a composite state with regions, exit all active region states first
	if len(e.activeConfig.regionStates) > 0 {
		for _, regionState := range e.activeConfig.regionStates {
			if err := e.exitState(regionState); err != nil {
				return fmt.Errorf("exit region state: %w", err)
			}
		}
		// Clear region configuration
		e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
	}
	
	// Record trace
	if e.trace != nil {
		e.trace.RecordStateExit(state.Name, len(state.Exit) > 0)
	}
	
	// Execute exit actions
	for _, action := range state.Exit {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("exit action: %w", err)
		}
	}
	
	// Clear simple state
	e.activeConfig.simpleState = nil
	
	return nil
}

// executeAction executes a single action (used for entry/exit/effect actions).
func (e *StateExecutor) executeAction(action ast.Node) error {
	actionNode, ok := action.(*ast.ActionExecutionNode)
	if !ok {
		return fmt.Errorf("unsupported action type: %T", action)
	}
	
	if actionNode.Expression != nil {
		// Evaluate inline expression
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData) // Make state data available
		result, err := ec.Eval(actionNode.Expression)
		if err != nil {
			return fmt.Errorf("eval expression: %w", err)
		}
		// Store result in state data with action name
		e.stateData[actionNode.Name] = result
	} else if actionNode.ActionRef != nil {
		return fmt.Errorf("nested action invocation not yet implemented")
	}
	
	return nil
}

// pollChangeEvents checks ChangeEvent conditions for outgoing transitions.
// Fires transition immediately if condition becomes true.
func (e *StateExecutor) pollChangeEvents() error {
	currentState := e.getCurrentState()
	if currentState == nil {
		return nil // Multi-region state, skip for now
	}
	transitions := e.transitions[currentState]
	
	for _, trans := range transitions {
		if changeEvent, ok := trans.Trigger.(*ast.ChangeEvent); ok {
			// Evaluate condition
			ec := NewEvalContext(e.ctx, nil)
			ec.Push(e.stateData)
			condVal, err := ec.Eval(changeEvent.Condition)
			if err != nil {
				return fmt.Errorf("eval change condition: %w", err)
			}
			
			// Check if boolean true
			isTrueVal := false
			if condVal.Kind == ValConst && condVal.Const.Kind == semantics.ValBool {
				isTrueVal = condVal.Const.Bool
			} else {
				return fmt.Errorf("change condition must be boolean, got %v", condVal.Kind)
			}
			
			// Fire transition if true
			if isTrueVal {
				return e.fireTransition(trans)
			}
		}
	}
	
	return nil
}

// --- Public accessor methods for REPL debugging ---

// CurrentState returns the current active state node.
func (e *StateExecutor) CurrentState() ast.Node {
	// For backward compatibility: return simple state if no regions active
	if e.activeConfig.simpleState != nil {
		return e.activeConfig.simpleState
	}
	// If multi-region, return nil (caller should check regions individually)
	return nil
}

// getCurrentState returns the active simple state (nil if multi-region).
func (e *StateExecutor) getCurrentState() *ast.StateNode {
	return e.activeConfig.simpleState
}

// setCurrentState sets the active simple state (for non-region states).
func (e *StateExecutor) setCurrentState(state *ast.StateNode) {
	e.activeConfig.simpleState = state
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode) // Clear regions
}

// evaluateChoicePseudostate evaluates a choice pseudostate dynamically.
// Returns the target state based on guard evaluation at runtime.
func (e *StateExecutor) evaluateChoicePseudostate(choice *ast.PseudostateNode) (*ast.StateNode, error) {
	// Find all outgoing transitions from this choice
	outgoing := e.findTransitionsFromPseudostate(choice)
	if len(outgoing) == 0 {
		return nil, fmt.Errorf("choice %s has no outgoing transitions", choice.Name)
	}
	
	// Evaluate guards dynamically in order
	for _, trans := range outgoing {
		if trans.Guard == nil {
			// Else branch (unguarded transition)
			targetState := e.findStateByName(trans.Target)
			if targetState == nil {
				targetName := "<unknown>"
				if trans.Target != nil && len(trans.Target.Parts) > 0 {
					targetName = trans.Target.Parts[len(trans.Target.Parts)-1].Text
				}
				return nil, fmt.Errorf("choice target state not found: %s", targetName)
			}
			return targetState, nil
		}
		
		// Evaluate guard
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData)
		guardVal, err := ec.Eval(trans.Guard)
		if err != nil {
			return nil, fmt.Errorf("eval choice guard: %w", err)
		}
		
		// Check boolean
		if guardVal.Kind == ValConst && guardVal.Const.Kind == semantics.ValBool && guardVal.Const.Bool {
			targetState := e.findStateByName(trans.Target)
			if targetState == nil {
				targetName := "<unknown>"
				if trans.Target != nil && len(trans.Target.Parts) > 0 {
					targetName = trans.Target.Parts[len(trans.Target.Parts)-1].Text
				}
				return nil, fmt.Errorf("choice target state not found: %s", targetName)
			}
			return targetState, nil
		}
	}
	
	return nil, fmt.Errorf("choice %s: no guard evaluated to true", choice.Name)
}

// evaluateJunctionPseudostate evaluates a junction pseudostate statically.
// Returns the target state based on static guard evaluation (pre-evaluated).
func (e *StateExecutor) evaluateJunctionPseudostate(junction *ast.PseudostateNode) (*ast.StateNode, error) {
	// Find all outgoing transitions from this junction
	outgoing := e.findTransitionsFromPseudostate(junction)
	if len(outgoing) == 0 {
		return nil, fmt.Errorf("junction %s has no outgoing transitions", junction.Name)
	}
	
	// Evaluate guards statically in order
	for _, trans := range outgoing {
		if trans.Guard == nil {
			// Else branch (unguarded transition)
			targetState := e.findStateByName(trans.Target)
			if targetState == nil {
				targetName := "<unknown>"
				if trans.Target != nil && len(trans.Target.Parts) > 0 {
					targetName = trans.Target.Parts[len(trans.Target.Parts)-1].Text
				}
				return nil, fmt.Errorf("junction target state not found: %s", targetName)
			}
			return targetState, nil
		}
		
		// Static evaluation (same as dynamic for now, but conceptually earlier)
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData)
		guardVal, err := ec.Eval(trans.Guard)
		if err != nil {
			return nil, fmt.Errorf("eval junction guard: %w", err)
		}
		
		// Check boolean
		if guardVal.Kind == ValConst && guardVal.Const.Kind == semantics.ValBool && guardVal.Const.Bool {
			targetState := e.findStateByName(trans.Target)
			if targetState == nil {
				targetName := "<unknown>"
				if trans.Target != nil && len(trans.Target.Parts) > 0 {
					targetName = trans.Target.Parts[len(trans.Target.Parts)-1].Text
				}
				return nil, fmt.Errorf("junction target state not found: %s", targetName)
			}
			return targetState, nil
		}
	}
	
	return nil, fmt.Errorf("junction %s: no guard evaluated to true", junction.Name)
}

// findTransitionsFromPseudostate finds all outgoing transitions from a pseudostate.
func (e *StateExecutor) findTransitionsFromPseudostate(ps *ast.PseudostateNode) []*ast.TransitionEdge {
	// Look up transitions from graph (already lowered)
	lowerTransitions, ok := e.graph.Transitions[ps]
	if !ok {
		return nil
	}
	
	// Convert lower.Transition back to TransitionEdge for compatibility
	result := make([]*ast.TransitionEdge, 0, len(lowerTransitions))
	for _, lowerTrans := range lowerTransitions {
		sourceName := getNodeName(lowerTrans.Source)
		targetName := getNodeName(lowerTrans.Target)
		
		edge := &ast.TransitionEdge{
			Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: sourceName}}},
			Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: targetName}}},
			Guard:   lowerTrans.Guard,
			Effect:  lowerTrans.Effect,
		}
		
		if lowerTrans.Trigger != nil {
			if triggerEvent, ok := lowerTrans.Trigger.(ast.TriggerEvent); ok {
				edge.Trigger = triggerEvent
			}
		}
		
		result = append(result, edge)
	}
	
	return result
}

// isMultiRegion checks if currently in a composite state with regions.
func (e *StateExecutor) isMultiRegion() bool {
	return len(e.activeConfig.regionStates) > 0
}

// StateStack returns a copy of the state stack (active configuration).
func (e *StateExecutor) StateStack() []*ast.StateNode {
	stack := make([]*ast.StateNode, len(e.stateStack))
	copy(stack, e.stateStack)
	return stack
}

// StateData returns a copy of state machine local data.
func (e *StateExecutor) StateData() map[string]Value {
	data := make(map[string]Value, len(e.stateData))
	for k, v := range e.stateData {
		data[k] = v
	}
	return data
}

// SetTrace sets the trace recorder for this executor.
func (e *StateExecutor) SetTrace(trace *TraceRecorder) {
	e.trace = trace
}

// EventQueue returns the event queue (not copied - read-only access).
func (e *StateExecutor) EventQueue() *EventQueue {
	return e.eventQueue
}

// CurrentTime returns the current simulation time.
func (e *StateExecutor) CurrentTime() float64 {
	return e.currentTime
}

// State returns current execution state.
func (e *StateExecutor) State() ExecutionState {
	return e.state
}

// StateMachineSymbol returns the state machine being executed.
func (e *StateExecutor) StateMachineSymbol() *symbols.Symbol {
	return e.stateMachine
}

// ProcessNextEvent processes the next event from the queue (for REPL stepping).
func (e *StateExecutor) ProcessNextEvent() error {
	return e.processNextEvent()
}
