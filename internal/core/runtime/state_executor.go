package runtime

import (
	"fmt"
	"math"

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
	activeConfig *StateConfiguration // Active state configuration (simple or multi-region)
	currentTime  float64
	nextEventID  int64 // Monotonic counter for unique event IDs
	eventQueue   *EventQueue
	stateData    map[string]Value // State machine local variables
	stateVisits  []string         // Ordered list of visited state names
	stateStack   []*ast.StateNode // Active state configuration (for nested states)
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
		ctx:          ctx,
		stateMachine: stateMachine,
		state:        StateReady,
		graph:        graph,
		currentTime:  0.0,
		nextEventID:  1,
		eventQueue:   NewEventQueue(),
		stateData:    make(map[string]Value),
		stateVisits:  make([]string, 0),
		stateStack:   make([]*ast.StateNode, 0),
		activeConfig: &StateConfiguration{
			regionStates: make(map[*ast.StateRegion]*ast.StateNode),
		},
	}

	// Initialize state machine attributes
	if err := exec.initializeAttributes(); err != nil {
		return nil, err
	}

	return exec, nil
}

// initializeAttributes populates stateData with attribute default values from state machine.
func (e *StateExecutor) initializeAttributes() error {
	// Get state machine node
	var members []ast.Node
	if usage, ok := e.stateMachine.Decl.(*ast.Usage); ok {
		members = usage.Members
	} else if def, ok := e.stateMachine.Decl.(*ast.Definition); ok {
		members = def.Members
	} else {
		return fmt.Errorf("state machine symbol has invalid node type")
	}

	// Extract attribute defaults
	for _, member := range members {
		// Unwrap Membership if present
		actualMember := member
		if membership, ok := member.(*ast.Membership); ok {
			actualMember = membership.Member
		}

		// Check for attribute with value
		if usage, ok := actualMember.(*ast.Usage); ok && usage.Kind == ast.UsageAttribute {
			if usage.Value != nil && usage.Ident.Name != "" {
				// Evaluate default value
				ec := NewEvalContext(e.ctx, nil)
				value, err := ec.Eval(usage.Value)
				if err != nil {
					return fmt.Errorf("eval attribute default %s: %w", usage.Ident.Name, err)
				}
				e.stateData[usage.Ident.Name] = value
			}
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
		parent, hasParent := e.graph.ParentState[current]
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
	for _, state := range e.graph.States {
		if state.Name == targetName {
			return state
		}
	}

	return nil
}

// scheduleTransitionEvents schedules TimeEvents for outgoing transitions from current state.
func (e *StateExecutor) scheduleTransitionEvents() error {
	currentState := e.getCurrentState()

	// Handle orthogonal regions separately
	if currentState == nil && len(e.activeConfig.regionStates) > 0 {
		// Multi-region state - schedule for each region
		for _, regionState := range e.activeConfig.regionStates {
			if err := e.scheduleTransitionsForState(regionState); err != nil {
				return err
			}
		}
		return nil
	}

	if currentState == nil {
		return nil // No active state
	}

	return e.scheduleTransitionsForState(currentState)
}

// scheduleTransitionsForState schedules events for outgoing transitions of a specific state.
func (e *StateExecutor) scheduleTransitionsForState(state *ast.StateNode) error {
	transitions := e.graph.Transitions[state]

	for _, trans := range transitions {
		if trans.Trigger == nil {
			// Completion transition - only schedule if guard is satisfied
			guardSatisfied := true
			if trans.Guard != nil {
				ec := NewEvalContext(e.ctx, nil)
				ec.Push(e.stateData)
				guardVal, err := ec.Eval(trans.Guard)
				if err != nil {
					return fmt.Errorf("eval completion guard: %w", err)
				}

				if guardVal.Kind == ValConst && guardVal.Const.Kind == semantics.ValBool {
					guardSatisfied = guardVal.Const.Bool
				} else {
					return fmt.Errorf("guard must be boolean, got %v", guardVal.Kind)
				}
			}

			if guardSatisfied {
				e.eventQueue.Push(Event{
					ID:        e.nextEventID,
					Type:      EventTime, // Use EventTime with nil trigger
					Timestamp: e.currentTime,
					Payload:   trans,
				})
				e.nextEventID++
			}
		} else if timeEvent, ok := trans.Trigger.(*ast.TimeEvent); ok {
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

			// `accept at t` names an instant, `accept after d` an offset from
			// entering the state. An instant already past fires immediately.
			timestamp := e.currentTime + duration
			if timeEvent.Absolute {
				timestamp = math.Max(duration, e.currentTime)
			}

			// Schedule event (generate unique ID using current queue length)
			e.eventQueue.Push(Event{
				ID:        e.nextEventID,
				Type:      EventTime,
				Timestamp: timestamp,
				Payload:   trans, // Store transition reference
			})
			e.nextEventID++
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
		// Fire transition - handle both old (TransitionEdge) and new (lower.Transition) for backward compatibility
		if lowerTrans, ok := event.Payload.(*lower.Transition); ok {
			// Determine if this transition is within a region
			if len(e.activeConfig.regionStates) > 0 {
				// Multi-region machine - find which region this transition belongs to
				var sourceState *ast.StateNode
				if stateNode, ok := lowerTrans.Source.(*ast.StateNode); ok {
					sourceState = stateNode
				}

				// A fork or join rewrites the whole active configuration, so a
				// transition into one is not region-local. Every other target
				// stays region-local, where fireTransitionInRegion handles it or
				// rejects it — routing it out of the region instead would drop
				// the sibling regions silently.
				if sourceState != nil && !isSynchronizationTarget(lowerTrans.Target) {
					// Check if this state is active in any region
					for region, activeState := range e.activeConfig.regionStates {
						if activeState == sourceState {
							// Found the region - fire transition within it
							return e.fireTransitionInRegion(region, lowerTrans)
						}
					}
				}
			}
			// Simple machine or couldn't determine region - use regular fireTransition
			return e.fireTransition(lowerTrans)
		}

		// Fallback for tests that use TransitionEdge directly
		if edge, ok := event.Payload.(*ast.TransitionEdge); ok {
			// Convert TransitionEdge to lower.Transition
			// Need to find source/target states by name
			var sourceState, targetState *ast.StateNode
			for _, state := range e.graph.States {
				if edge.Source != nil && len(edge.Source.Parts) > 0 {
					if state.Name == edge.Source.Parts[len(edge.Source.Parts)-1].Text {
						sourceState = state
					}
				}
				if edge.Target != nil && len(edge.Target.Parts) > 0 {
					if state.Name == edge.Target.Parts[len(edge.Target.Parts)-1].Text {
						targetState = state
					}
				}
			}
			if sourceState == nil || targetState == nil {
				return fmt.Errorf("could not find source/target states for transition")
			}

			lowerTrans := &lower.Transition{
				Source:  sourceState,
				Target:  targetState,
				Trigger: edge.Trigger,
				Guard:   edge.Guard,
				Effect:  edge.Effect,
			}
			return e.fireTransition(lowerTrans)
		}

		return fmt.Errorf("invalid TimeEvent payload: expected *lower.Transition or *ast.TransitionEdge")
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
			transitions := e.graph.Transitions[regionState]
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

	transitions := e.graph.Transitions[currentState]
	for _, trans := range transitions {
		if e.matchesEvent(trans, event) {
			return e.fireTransition(trans)
		}
	}

	return nil // Event not consumed
}

// matchesEvent checks if a transition matches the given event.
func (e *StateExecutor) matchesEvent(trans *lower.Transition, event *Event) bool {
	// Completion transition (nil trigger) doesn't match external events
	if trans.Trigger == nil {
		return false
	}

	switch event.Type {
	case EventAccept:
		// Must have AcceptEvent trigger
		acceptEvent, ok := trans.Trigger.(*ast.AcceptEvent)
		if !ok {
			return false
		}

		// Extract message payload
		msg, ok := event.Payload.(Message)
		if !ok {
			return false
		}

		// Match signal type (last segment of QualifiedName against Message.SignalType)
		expectedSignal := ast.SimpleName(acceptEvent.SignalType)
		if expectedSignal == "" {
			return false
		}
		return msg.carriesSignal(expectedSignal)

	case EventCall:
		// Must have CallEvent trigger
		callEvent, ok := trans.Trigger.(*ast.CallEvent)
		if !ok {
			return false
		}
		call, ok := event.Payload.(Call)
		if !ok {
			return false
		}
		// A trigger naming an operation fires only for that operation; a trigger
		// naming none fires for any call.
		expectedOp := ast.SimpleName(callEvent.Operation)
		return expectedOp == "" || expectedOp == call.Operation

	case EventChange:
		// Must have ChangeEvent trigger
		changeEvent, ok := trans.Trigger.(*ast.ChangeEvent)
		if !ok {
			return false
		}
		// Re-evaluate condition (pollChangeEvents is the primary driver)
		// Here we just verify it's a ChangeEvent trigger
		return changeEvent.Condition != nil

	case EventTime:
		// Time events carry the specific transition in Payload
		// If matchesEvent is called for time events (shouldn't normally happen),
		// match if this transition is the one in the payload
		if transPayload, ok := event.Payload.(*lower.Transition); ok {
			return trans == transPayload
		}
		return false

	default:
		return false
	}
}

// fireTransitionInRegion fires a transition within a specific region.
func (e *StateExecutor) fireTransitionInRegion(region *ast.StateRegion, trans *lower.Transition) error {
	// Target should be a StateNode (pseudostates not supported in region transitions yet)
	targetState, ok := trans.Target.(*ast.StateNode)
	if !ok {
		return fmt.Errorf("transition target must be a state node, got %T", trans.Target)
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

	// Schedule outgoing transitions from the new state
	if err := e.scheduleTransitionsForState(targetState); err != nil {
		return fmt.Errorf("schedule transitions: %w", err)
	}

	// Record trace
	if e.trace != nil {
		eventName := triggerName(trans.Trigger)
		e.trace.RecordStateTransition(sourceState.Name, targetState.Name, eventName)
	}

	return nil
}

// fireTransition executes a state transition.
func (e *StateExecutor) fireTransition(trans *lower.Transition) error {
	// Fork and join reshape the active configuration rather than moving to a
	// single state, so they are fired whole.
	if ps, ok := trans.Target.(*ast.PseudostateNode); ok {
		switch ps.Kind {
		case ast.PseudostateFork:
			return e.fireForkTransition(trans, ps)
		case ast.PseudostateJoin:
			return e.fireJoinTransition(trans, ps)
		}
	}

	// Target can be StateNode or PseudostateNode
	var targetState *ast.StateNode

	// Type assert to determine target type
	switch target := trans.Target.(type) {
	case *ast.StateNode:
		targetState = target
	case *ast.PseudostateNode:
		// Evaluate pseudostate to get final target state
		var err error
		switch target.Kind {
		case ast.PseudostateChoice:
			targetState, err = e.evaluateChoicePseudostate(target)
		case ast.PseudostateJunction, ast.PseudostateEntry, ast.PseudostateExit:
			// Entry and exit points name a boundary crossing: the transition
			// continues along the point's own outgoing transition.
			targetState, err = e.evaluateJunctionPseudostate(target)
		default:
			return fmt.Errorf("unsupported pseudostate kind: %v", target.Kind)
		}
		if err != nil {
			return fmt.Errorf("evaluate pseudostate: %w", err)
		}
	default:
		return fmt.Errorf("transition target must be StateNode or PseudostateNode, got %T", trans.Target)
	}

	if targetState == nil {
		return fmt.Errorf("transition target state not found")
	}

	pass, err := e.passesGuard(trans.Guard)
	if err != nil {
		return err
	}
	if !pass {
		return nil // Remain in current state
	}

	return e.transitionTo(trans, targetState)
}

// transitionTo moves the active configuration from the current state to
// targetState: exit up to the least common ancestor, run the transition effect,
// then enter down to the target.
func (e *StateExecutor) transitionTo(trans *lower.Transition, targetState *ast.StateNode) error {
	// Exit current state hierarchy up to LCA
	currentState := e.getCurrentState()
	// The trace's source name has to be read before the move, not after it.
	fromName := ""
	if currentState != nil {
		fromName = currentState.Name
	}
	lca := e.getLCA(currentState, targetState)
	statesToExit := make([]*ast.StateNode, 0)
	current := currentState
	for current != nil && current != lca {
		statesToExit = append(statesToExit, current)
		current = e.graph.ParentState[current]
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
		current = e.graph.ParentState[current]
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
		eventName := triggerName(trans.Trigger)
		e.trace.RecordStateTransition(fromName, targetState.Name, eventName)
	}

	return nil
}

// isSynchronizationTarget reports whether a transition target is a fork or join
// pseudostate, the two kinds that replace the entire active configuration.
func isSynchronizationTarget(target ast.Node) bool {
	ps, ok := target.(*ast.PseudostateNode)
	return ok && (ps.Kind == ast.PseudostateFork || ps.Kind == ast.PseudostateJoin)
}

// passesGuard reports whether a transition's guard allows it to fire. A nil
// guard always passes.
func (e *StateExecutor) passesGuard(guard ast.Node) (bool, error) {
	if guard == nil {
		return true, nil
	}
	ec := NewEvalContext(e.ctx, nil)
	ec.Push(e.stateData)
	val, err := ec.Eval(guard)
	if err != nil {
		return false, fmt.Errorf("eval guard: %w", err)
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("guard must be boolean, got %v", val.Kind)
	}
	return val.Const.Bool, nil
}

// fireForkTransition takes a transition into a fork: every outgoing branch is
// taken at once, making one state active per orthogonal region of the composite
// state that owns them.
func (e *StateExecutor) fireForkTransition(trans *lower.Transition, fork *ast.PseudostateNode) error {
	pass, err := e.passesGuard(trans.Guard)
	if err != nil || !pass {
		return err
	}

	branches := e.graph.Transitions[fork]
	if len(branches) < 2 {
		return fmt.Errorf("fork %s needs at least two outgoing transitions, found %d", fork.Name, len(branches))
	}

	targets := make(map[*ast.StateRegion]*ast.StateNode, len(branches))
	var owner *ast.StateNode
	for _, branch := range branches {
		if branch.Guard != nil {
			return fmt.Errorf("fork %s: outgoing transitions cannot be guarded", fork.Name)
		}
		target, ok := branch.Target.(*ast.StateNode)
		if !ok {
			return fmt.Errorf("fork %s: branch target must be a state, got %T", fork.Name, branch.Target)
		}
		region, ok := e.graph.RegionOf[target]
		if !ok {
			return fmt.Errorf("fork %s: branch target %s is not in an orthogonal region", fork.Name, target.Name)
		}
		if existing, dup := targets[region]; dup {
			return fmt.Errorf("fork %s: branches %s and %s are in the same region", fork.Name, existing.Name, target.Name)
		}
		targets[region] = target
		if regionOwner := e.graph.RegionOwner[region]; regionOwner != nil {
			if owner != nil && owner != regionOwner {
				return fmt.Errorf("fork %s: branches span more than one composite state", fork.Name)
			}
			owner = regionOwner
		}
	}

	// Leave the source configuration, up to but excluding the composite state
	// the branches live in.
	if err := e.exitToward(owner); err != nil {
		return err
	}
	for _, action := range trans.Effect {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}

	// Enter the composite state's own hierarchy, activating each targeted region
	// at its branch target rather than at the region's initial state: an explicit
	// fork bypasses the initial pseudostate, so that state's entry and do
	// behaviors must not run. Regions the fork does not target start normally.
	if owner == nil {
		return fmt.Errorf("fork %s: branch targets have no owning composite state", fork.Name)
	}
	if err := e.enterHierarchyInto(owner, targets); err != nil {
		return err
	}

	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}
	if e.trace != nil {
		e.trace.RecordStateTransition(getNodeName(trans.Source), fork.Name, "")
	}
	return nil
}

// fireJoinTransition takes a transition into a join. The join only fires once
// every one of its incoming branches has an active source state; until then the
// completed branch simply waits.
func (e *StateExecutor) fireJoinTransition(trans *lower.Transition, join *ast.PseudostateNode) error {
	pass, err := e.passesGuard(trans.Guard)
	if err != nil || !pass {
		return err
	}

	sources, err := e.joinSources(join)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if !e.isActive(source) {
			return nil // Not yet synchronized: wait for the other branches.
		}
	}

	target, err := e.evaluateJunctionPseudostate(join)
	if err != nil {
		return fmt.Errorf("evaluate pseudostate: %w", err)
	}
	if target == nil {
		return fmt.Errorf("join %s has no target state", join.Name)
	}

	// Exit every synchronized branch, then continue from the composite state
	// they belong to so the usual hierarchy walk exits it as well.
	var owner *ast.StateNode
	for _, source := range sources {
		if region, ok := e.graph.RegionOf[source]; ok {
			owner = e.graph.RegionOwner[region]
		}
		if err := e.exitState(source); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
	e.activeConfig.simpleState = owner

	return e.transitionTo(trans, target)
}

// joinSources returns the source state of every transition into join, in state
// declaration order. That order is observable — it is the order the branches are
// exited in — so it comes from graph.States rather than from the graph.Transitions
// map, whose iteration order varies between runs.
func (e *StateExecutor) joinSources(join *ast.PseudostateNode) ([]*ast.StateNode, error) {
	var sources []*ast.StateNode
	for _, state := range e.graph.States {
		for _, trans := range e.graph.Transitions[state] {
			if trans.Target == ast.Node(join) {
				sources = append(sources, state)
			}
		}
	}
	for _, ps := range e.graph.Pseudostates {
		for _, trans := range e.graph.Transitions[ps] {
			if trans.Target == ast.Node(join) {
				return nil, fmt.Errorf("join %s: incoming source must be a state, got %T", join.Name, trans.Source)
			}
		}
	}
	if len(sources) < 2 {
		return nil, fmt.Errorf("join %s needs at least two incoming transitions, found %d", join.Name, len(sources))
	}
	return sources, nil
}

// isActive reports whether state is part of the active configuration.
func (e *StateExecutor) isActive(state *ast.StateNode) bool {
	if e.activeConfig.simpleState == state {
		return true
	}
	for _, active := range e.activeConfig.regionStates {
		if active == state {
			return true
		}
	}
	return false
}

// exitToward exits the active configuration up to, but not including, stop.
func (e *StateExecutor) exitToward(stop *ast.StateNode) error {
	for _, active := range e.orderedRegionStates() {
		if err := e.exitState(active); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)

	for current := e.getCurrentState(); current != nil && current != stop; current = e.graph.ParentState[current] {
		if err := e.exitState(current); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	e.activeConfig.simpleState = nil
	return nil
}

// orderedRegionStates returns the active state of each orthogonal region in
// region declaration order, since exit behaviors run in the order returned and
// activeConfig.regionStates is a map.
func (e *StateExecutor) orderedRegionStates() []*ast.StateNode {
	states := make([]*ast.StateNode, 0, len(e.activeConfig.regionStates))
	seen := make(map[*ast.StateRegion]bool, len(e.activeConfig.regionStates))
	for _, state := range e.graph.States {
		for _, region := range e.graph.CompositeStates[state] {
			seen[region] = true
			if active, ok := e.activeConfig.regionStates[region]; ok {
				states = append(states, active)
			}
		}
	}
	// A region whose owner is absent from graph.States still has to be exited.
	for region, active := range e.activeConfig.regionStates {
		if !seen[region] {
			states = append(states, active)
		}
	}
	return states
}

// enterHierarchyInto enters state's ancestors, outermost first, then state
// itself, skipping any that are already active. State's own orthogonal regions
// start at the given branch targets wherever branches names one.
func (e *StateExecutor) enterHierarchyInto(state *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) error {
	chain := e.getParentChain(state)
	for i := len(chain) - 1; i >= 0; i-- {
		if e.isActive(chain[i]) {
			continue
		}
		var err error
		if chain[i] == state {
			err = e.enterStateInto(state, branches)
		} else {
			err = e.enterState(chain[i])
		}
		if err != nil {
			return fmt.Errorf("enter state %s: %w", chain[i].Name, err)
		}
	}
	return nil
}

// maxStateEvents bounds a single run of the event loop so a cyclic machine
// reports a typed error instead of spinning forever.
const maxStateEvents = 10000

// RunToCompletion processes queued events until the machine completes or has no
// event left to process, at which point it suspends.
func (e *StateExecutor) RunToCompletion() error {
	for events := 0; e.state == StateRunning; events++ {
		if e.eventQueue.Len() == 0 && !e.deliverPendingSignal() {
			e.state = StateSuspended
			return nil
		}
		if events >= maxStateEvents {
			return fmt.Errorf("state machine exceeded max events (%d), possible infinite loop", maxStateEvents)
		}
		if err := e.processNextEvent(); err != nil {
			return fmt.Errorf("process event: %w", err)
		}
	}
	return nil
}

// SendSignal injects a signal event into the state machine.
// This is the primary API for driving state machines with external signals.
// The signal is enqueued and will be processed on the next ProcessNextEvent call.
func (e *StateExecutor) SendSignal(signalType string, args map[string]Value) {
	e.enqueueSignal(Message{SignalType: signalType, Payload: args})
}

// InvokeOperation injects a call event for the named operation. Transitions
// triggered by that operation fire; transitions triggered by another do not.
func (e *StateExecutor) InvokeOperation(operation string, args map[string]Value) {
	e.eventQueue.Push(Event{
		ID:        e.nextEventID,
		Type:      EventCall,
		Timestamp: e.currentTime,
		Payload:   Call{Operation: operation, Args: args},
	})
	e.nextEventID++
}

// enqueueSignal queues a message as an accept event, to fire immediately.
func (e *StateExecutor) enqueueSignal(msg Message) {
	e.eventQueue.Push(Event{
		ID:        e.nextEventID,
		Type:      EventAccept,
		Timestamp: e.currentTime,
		Payload:   msg,
	})
	e.nextEventID++
}

// deliverPendingSignal takes a message off the context bus that the active
// configuration can react to and queues it, reporting whether it found one.
// A message no active transition accepts stays on the bus for another consumer:
// this machine must not swallow a message addressed to a different behavior.
func (e *StateExecutor) deliverPendingSignal() bool {
	msg, ok := e.ctx.TakeMessage(func(m Message) bool {
		// A transition trigger names a signal, never a port (`accept <type>` has
		// no `via` form), so a message routed to a port is not for this machine.
		return m.arrivedAt("") && m.addressedTo(e.stateMachine.Name) && e.acceptsSignal(m)
	})
	if !ok {
		return false
	}
	e.enqueueSignal(msg)
	return true
}

// acceptsSignal reports whether any transition out of the active configuration
// is triggered by this message's signal.
func (e *StateExecutor) acceptsSignal(msg Message) bool {
	for _, state := range e.activeStates() {
		for _, trans := range e.graph.Transitions[state] {
			accept, ok := trans.Trigger.(*ast.AcceptEvent)
			if !ok {
				continue
			}
			signal := ast.SimpleName(accept.SignalType)
			if signal != "" && msg.carriesSignal(signal) {
				return true
			}
		}
	}
	return false
}

// activeStates returns the states currently active: one per region for a
// composite configuration, otherwise the single active state.
func (e *StateExecutor) activeStates() []*ast.StateNode {
	if len(e.activeConfig.regionStates) > 0 {
		states := make([]*ast.StateNode, 0, len(e.activeConfig.regionStates))
		for _, state := range e.activeConfig.regionStates {
			states = append(states, state)
		}
		return states
	}
	if current := e.getCurrentState(); current != nil {
		return []*ast.StateNode{current}
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

	// Schedule events for outgoing transitions in all regions
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}

	return nil
}

// enterState executes entry behaviors when entering a state.
func (e *StateExecutor) enterState(state *ast.StateNode) error {
	return e.enterStateInto(state, nil)
}

// enterStateInto enters state, starting each of its orthogonal regions at
// branches[region] where branches names one and at the region's initial state
// otherwise. A fork supplies branches: it enters its targets directly instead of
// the regions' initial states, whose entry and do behaviors it bypasses.
func (e *StateExecutor) enterStateInto(state *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) error {
	if state == nil {
		return nil
	}

	// Track state visit
	e.stateVisits = append(e.stateVisits, state.Name)

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
	if regions, isComposite := e.graph.CompositeStates[state]; isComposite {
		// Entering composite state with orthogonal regions
		// Initialize active configuration for all regions
		e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
		e.activeConfig.simpleState = nil // Clear simple state

		for _, region := range regions {
			entry, targeted := branches[region]
			if !targeted {
				entry = e.graph.RegionInitials[region]
			}
			e.activeConfig.regionStates[region] = entry
			// Recursively enter each region's starting state
			if err := e.enterState(entry); err != nil {
				return fmt.Errorf("enter starting state in region %s: %w", region.Name, err)
			}
		}
	} else {
		// Simple state (no regions)
		// Only set simpleState if we're not in a multi-region machine
		// In multi-region machines, states belong to specific regions and simpleState should stay nil
		if len(e.activeConfig.regionStates) == 0 {
			e.activeConfig.simpleState = state
		}
		// Otherwise, the region state was already set by fireTransitionInRegion
	}

	// Execute do activities (ongoing behavior)
	// Simplified: execute immediately like entry actions
	// Full UML semantics: concurrent execution with state lifetime
	for _, action := range state.Do {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("do action: %w", err)
		}
	}

	// Don't schedule transitions here - let the caller decide when to schedule
	// This prevents double-scheduling in region transitions

	return nil
}

// exitState executes exit behaviors when leaving a state.
func (e *StateExecutor) exitState(state *ast.StateNode) error {
	if state == nil {
		return nil
	}

	// Check if this state is a composite state with regions
	regions, isComposite := e.graph.CompositeStates[state]

	// If this is a composite state with regions, exit all active region states first
	if isComposite && len(regions) > 0 && len(e.activeConfig.regionStates) > 0 {
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

// invokeNested performs an action from a state's entry/exit/effect behavior,
// passing state data in through the callee's input parameters and merging its
// output parameters back into state data.
func (e *StateExecutor) invokeNested(inv actionInvocation) error {
	outputs, err := invokeAction(e.ctx, e.stateMachine.Scope, inv, e.stateData)
	if err != nil {
		return err
	}
	for name, value := range outputs {
		e.stateData[name] = value
	}
	return nil
}

// executeAction executes a single action (used for entry/exit/effect actions).
func (e *StateExecutor) executeAction(action ast.Node) error {
	switch node := action.(type) {
	case *ast.ActionExecutionNode:
		if node.Expression != nil {
			// Evaluate inline expression
			ec := NewEvalContext(e.ctx, nil)
			ec.Push(e.stateData) // Make state data available
			result, err := ec.Eval(node.Expression)
			if err != nil {
				return fmt.Errorf("eval expression: %w", err)
			}
			// Store result in state data with action name
			e.stateData[node.Name] = result
		} else if node.ActionRef != nil {
			return e.invokeNested(actionInvocation{target: node.ActionRef})
		}
		return nil

	case *ast.Usage:
		// An entry/exit/effect action that performs another action:
		// perform X; / action a : X; / action a = X(...);
		// A state action never accepts on a port, so every reference it carries
		// names the action it performs.
		inv, ok := nestedInvocation(node, "")
		if !ok {
			return fmt.Errorf("state action %s performs no action", node.Ident.Name)
		}
		return e.invokeNested(inv)

	case *ast.AssignmentActionNode:
		// Handle assignment (e.g., counter = counter + 1)
		// Evaluate RHS
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData)
		rhsVal, err := ec.Eval(node.Value)
		if err != nil {
			return fmt.Errorf("eval assignment RHS: %w", err)
		}

		// Extract target name
		var targetName string
		switch target := node.Target.(type) {
		case *ast.QualifiedName:
			if len(target.Parts) == 1 {
				targetName = target.Parts[0].Text
			} else {
				return fmt.Errorf("qualified assignment target not supported: %v", target)
			}
		case *ast.FeatureReference:
			if target.Name != nil && len(target.Name.Parts) == 1 {
				targetName = target.Name.Parts[0].Text
			} else {
				return fmt.Errorf("qualified feature reference not supported: %v", target.Name)
			}
		default:
			return fmt.Errorf("unsupported assignment target type: %T", target)
		}

		// Store in state data
		e.stateData[targetName] = rhsVal
		return nil

	case *ast.SendStatement:
		// A send in an entry/do/exit/effect action posts to the context bus,
		// where this machine's own transitions and other behaviors can accept it.
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData)
		send := lower.Send{
			Message: node.Message,
			Target:  ast.SimpleName(node.Target),
			IsVia:   node.IsVia,
		}
		msg, err := ec.buildMessage(e.stateMachine.Scope, send)
		if err != nil {
			return err
		}
		e.ctx.post(e.graph.Connections, msg, send)
		return nil

	default:
		return fmt.Errorf("unsupported action type: %T", action)
	}
}

// pollChangeEvents checks ChangeEvent conditions for outgoing transitions.
// Fires transition immediately if condition becomes true.
func (e *StateExecutor) pollChangeEvents() error {
	currentState := e.getCurrentState()
	if currentState == nil {
		return nil // Multi-region state, skip for now
	}
	transitions := e.graph.Transitions[currentState]

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

// GetStateVisits returns the ordered list of visited state names.
func (e *StateExecutor) GetStateVisits() []string {
	return e.stateVisits
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
			Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: sourceName}}},
			Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: targetName}}},
			Guard:  lowerTrans.Guard,
			Effect: lowerTrans.Effect,
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
