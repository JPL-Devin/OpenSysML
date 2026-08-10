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

	// history records, per composite state, the configuration that state had when
	// it was last exited. A history pseudostate re-enters that configuration
	// instead of the composite state's initial one.
	history map[*ast.StateNode]*historyRecord

	// deferred holds, in arrival order, the events an active state defers and no
	// transition of the active configuration handled.
	deferred []Event

	// doActivities are the running do behaviors, in the order their states were
	// entered. Concurrently active states interleave one action per round, so this
	// order — not map iteration order — decides the interleaving.
	doActivities []*doActivity
}

// doActivity is the part of a state's do behavior that has still to run. The
// behavior runs while its state is active rather than at entry, and is abandoned
// when the state is exited.
type doActivity struct {
	state   *ast.StateNode
	pending []ast.Node
}

// historyRecord is the configuration one composite state was last left in.
type historyRecord struct {
	// child is the substate that was active, whether it was declared directly or
	// in one of the state's orthogonal regions.
	child *ast.StateNode
	// regions is the active state of each orthogonal region, empty for a state
	// that has none.
	regions map[*ast.StateRegion]*ast.StateNode
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
		history:      make(map[*ast.StateNode]*historyRecord),
		deferred:     make([]Event, 0),
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

// scheduleTransitionEvents schedules TimeEvents for outgoing transitions from current state.
func (e *StateExecutor) scheduleTransitionEvents() error {
	currentState := e.getCurrentState()

	// Handle orthogonal regions separately
	if currentState == nil && len(e.activeConfig.regionStates) > 0 {
		// Multi-region state - schedule for each region, in declaration order:
		// the order the regions' transitions are queued in is observable.
		for _, regionState := range e.orderedRegionStates() {
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

// scheduleCompletionTransitions queues the completion transitions of a state
// whose guard holds. A state completes only once its do behavior has finished,
// so a state still running one is skipped here and scheduled by runDoRound when
// the behavior ends.
func (e *StateExecutor) scheduleCompletionTransitions(state *ast.StateNode) error {
	if e.hasRunningDoActivity(state) {
		return nil
	}

	for _, trans := range e.graph.Transitions[state] {
		if trans.Trigger != nil {
			continue
		}
		satisfied, err := e.passesGuard(trans.Guard)
		if err != nil {
			return fmt.Errorf("eval completion guard: %w", err)
		}
		if !satisfied {
			continue
		}
		e.eventQueue.Push(Event{
			ID:        e.nextEventID,
			Type:      EventTime, // Use EventTime with nil trigger
			Timestamp: e.currentTime,
			Payload:   trans,
		})
		e.nextEventID++
	}
	return nil
}

// scheduleTransitionsForState schedules events for outgoing transitions of a specific state.
func (e *StateExecutor) scheduleTransitionsForState(state *ast.StateNode) error {
	transitions := e.graph.Transitions[state]

	if err := e.scheduleCompletionTransitions(state); err != nil {
		return err
	}

	for _, trans := range transitions {
		if trans.Trigger == nil {
			continue // scheduled above, once the state's do behavior has finished
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

// processNextEvent pops and processes the next event from queue. It is one
// run-to-completion step: the event is dispatched, and only once it has been
// fully handled are events the new configuration no longer defers dispatched
// again.
func (e *StateExecutor) processNextEvent() error {
	if e.eventQueue.Len() == 0 {
		return fmt.Errorf("no events to process")
	}

	event := e.eventQueue.Pop()

	// Advance time
	e.currentTime = event.Timestamp

	if err := e.dispatchEvent(event); err != nil {
		return err
	}
	e.recallDeferredEvents()
	return nil
}

// dispatchEvent delivers one event to the active configuration.
func (e *StateExecutor) dispatchEvent(event Event) error {
	// Process event by type
	switch event.Type {
	case EventTime:
		// Fire transition - handle both old (TransitionEdge) and new (lower.Transition) for backward compatibility
		if lowerTrans, ok := event.Payload.(*lower.Transition); ok {
			sourceState, _ := lowerTrans.Source.(*ast.StateNode)
			if sourceState != nil && !e.inActiveConfiguration(sourceState) {
				// The source was left before this event came up, so the transition
				// it carries is stale: firing it would move a state machine that is
				// no longer there.
				return nil
			}
			// A transition out of a state that is the active state of an orthogonal
			// region is region-local: it must not tear down the sibling regions
			// unless its target lies outside the region set.
			if sourceState != nil {
				for _, region := range e.orderedActiveRegions() {
					if e.activeConfig.regionStates[region] == sourceState {
						return e.fireTransitionInRegion(region, lowerTrans)
					}
				}
			}
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
		consumed, err := e.broadcastEvent(&event)
		if err != nil {
			return err
		}
		if !consumed && e.defersEvent(&event) {
			e.deferred = append(e.deferred, event)
		}
		return nil
	}
}

// defersEvent reports whether any state of the active configuration, or an
// ancestor of one, defers this event. A composite state's deferral holds while
// any of its substates is active.
func (e *StateExecutor) defersEvent(event *Event) bool {
	for _, state := range e.activeStates() {
		for _, ancestor := range e.getParentChain(state) {
			for _, trigger := range e.graph.Deferred[ancestor] {
				if triggerMatches(trigger, event) {
					return true
				}
			}
		}
	}
	return false
}

// recallDeferredEvents returns every deferred event the configuration reached by
// the step just finished no longer defers to the event pool. A recalled event
// keeps its ID, so it is dispatched ahead of whatever arrived while it was held
// back, but not its original timestamp, which would move virtual time backwards.
func (e *StateExecutor) recallDeferredEvents() {
	if len(e.deferred) == 0 {
		return
	}
	retained := make([]Event, 0, len(e.deferred))
	for _, event := range e.deferred {
		if e.defersEvent(&event) {
			retained = append(retained, event)
			continue
		}
		event.Timestamp = e.currentTime
		e.eventQueue.Push(event)
	}
	e.deferred = retained
}

// broadcastEvent sends an event to all active regions (or single state if no
// regions), reporting whether any of them consumed it. An event no region
// consumed is either deferred or dropped by the caller, so "a transition fired"
// and "nothing happened" must not look alike here.
func (e *StateExecutor) broadcastEvent(event *Event) (bool, error) {
	// If in composite state with regions, broadcast to all
	if len(e.activeConfig.regionStates) > 0 {
		// Broadcast the event to each region independently, in declaration order.
		// A region may be left by another region's reaction, so its active state is
		// read again here rather than snapshotted.
		consumed := false
		for _, region := range e.orderedActiveRegions() {
			regionState, stillActive := e.activeConfig.regionStates[region]
			if !stillActive {
				continue
			}
			trans, err := e.enabledTransition(regionState, event)
			if err != nil {
				return consumed, fmt.Errorf("region %s: %w", region.Name, err)
			}
			if trans == nil {
				continue
			}
			// Run-to-completion: one transition per region per event.
			if err := e.fireTransitionInRegion(region, trans); err != nil {
				return consumed, fmt.Errorf("fire transition in region %s: %w", region.Name, err)
			}
			consumed = true
		}
		return consumed, nil
	}

	// Simple state (no regions) - existing logic
	currentState := e.getCurrentState()
	if currentState == nil {
		return false, nil // No active state
	}

	trans, err := e.enabledTransition(currentState, event)
	if err != nil || trans == nil {
		return false, err
	}
	if err := e.fireTransition(trans); err != nil {
		return false, err
	}
	return true, nil
}

// enabledTransition returns the first transition out of state that this event
// triggers and whose guard holds, or nil when the state cannot react to it. A
// transition whose guard is false does not consume the event, so a later one
// still gets its chance.
func (e *StateExecutor) enabledTransition(state *ast.StateNode, event *Event) (*lower.Transition, error) {
	for _, trans := range e.graph.Transitions[state] {
		if !e.matchesEvent(trans, event) {
			continue
		}
		// A call trigger's arguments are bound before the guard runs: the guard is
		// written against the parameters the trigger declares. A transition that
		// does not fire must leave no trace of them in the machine's data.
		unbind, err := e.bindTriggerArguments(trans, event)
		if err != nil {
			unbind()
			return nil, err
		}
		pass, err := e.passesGuard(trans.Guard)
		if err != nil {
			unbind()
			return nil, err
		}
		if pass {
			return trans, nil
		}
		unbind()
	}
	return nil, nil
}

// bindTriggerArguments binds the parameters a call trigger declares to the
// arguments of the invocation, so the transition's guard and effect can read
// them. It returns the function restoring the machine's data to what it held
// before, for the caller to run when the transition does not fire.
func (e *StateExecutor) bindTriggerArguments(trans *lower.Transition, event *Event) (func(), error) {
	callEvent, ok := trans.Trigger.(*ast.CallEvent)
	if !ok || len(callEvent.Parameters) == 0 {
		return func() {}, nil
	}
	unbind := e.restoreData(callEvent.Parameters)
	call, ok := event.Payload.(Call)
	if !ok {
		return unbind, fmt.Errorf("call trigger %s: event carries %T, not an operation invocation",
			ast.SimpleName(callEvent.Operation), event.Payload)
	}
	for _, param := range callEvent.Parameters {
		value, ok := call.Args[param.Text]
		if !ok {
			return unbind, fmt.Errorf("call trigger %s: invocation carries no argument %q",
				call.Operation, param.Text)
		}
		e.stateData[param.Text] = value
	}
	return unbind, nil
}

// restoreData snapshots the named entries of the machine's data and returns the
// function putting them back, deleting the ones that were not there before.
func (e *StateExecutor) restoreData(names []ast.NameSegment) func() {
	saved := make(map[string]Value, len(names))
	held := make(map[string]bool, len(names))
	for _, name := range names {
		value, ok := e.stateData[name.Text]
		saved[name.Text], held[name.Text] = value, ok
	}
	return func() {
		for name, wasHeld := range held {
			if wasHeld {
				e.stateData[name] = saved[name]
			} else {
				delete(e.stateData, name)
			}
		}
	}
}

// matchesEvent checks if a transition matches the given event.
func (e *StateExecutor) matchesEvent(trans *lower.Transition, event *Event) bool {
	// Completion transition (nil trigger) doesn't match external events
	if trans.Trigger == nil {
		return false
	}

	switch event.Type {
	case EventAccept, EventCall, EventChange:
		return triggerMatches(trans.Trigger, event)

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

// triggerMatches reports whether a trigger reacts to an event, whether the
// trigger belongs to a transition or to a state's deferred set.
func triggerMatches(trigger ast.Node, event *Event) bool {
	switch event.Type {
	case EventAccept:
		acceptEvent, ok := trigger.(*ast.AcceptEvent)
		if !ok {
			return false
		}
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
		callEvent, ok := trigger.(*ast.CallEvent)
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
		if expectedOp != "" && expectedOp != call.Operation {
			return false
		}
		// A trigger declaring parameters fires only for a call carrying an
		// argument of each declared name; `op()` takes the call whatever it carries.
		for _, param := range callEvent.Parameters {
			if _, ok := call.Args[param.Text]; !ok {
				return false
			}
		}
		return true

	case EventChange:
		// Re-evaluate condition (pollChangeEvents is the primary driver); here we
		// just verify it is a change trigger with a condition.
		changeEvent, ok := trigger.(*ast.ChangeEvent)
		return ok && changeEvent.Condition != nil

	default:
		return false
	}
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
		case ast.PseudostateShallowHistory, ast.PseudostateDeepHistory:
			return e.fireHistoryTransition(trans, ps)
		}
	}

	// Target can be StateNode or PseudostateNode
	var targetState *ast.StateNode

	// Type assert to determine target type
	switch target := trans.Target.(type) {
	case *ast.StateNode:
		targetState = target
	case *ast.PseudostateNode:
		// A choice, junction or entry/exit point routes the transition onwards:
		// following it yields the state the transition really enters.
		if !transientPseudostate(target.Kind) {
			return fmt.Errorf("unsupported pseudostate kind: %v", target.Kind)
		}
		var err error
		targetState, err = e.pseudostateTarget(target)
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
	return e.transitionToInto(trans, targetState, nil)
}

// transitionToInto is transitionTo with branches naming the state each
// orthogonal region entered on the way must start in, which is how a history
// pseudostate restores a recorded configuration rather than the initial one.
func (e *StateExecutor) transitionToInto(trans *lower.Transition, targetState *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) error {
	// Exit current state hierarchy up to LCA
	currentState := e.getCurrentState()
	if currentState == nil {
		// A composite state with orthogonal regions is represented by its
		// regions' active states rather than by a single active state, so leaving
		// it starts from the state that owns those regions.
		currentState = e.activeCompositeOwner()
	}
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
		if err := e.enterStateInto(statesToEnter[i], branches); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
	}

	// Update current state and rebuild stateStack with full active configuration.
	// A composite state with orthogonal regions is represented by its regions'
	// active states, which entering it has just filled in, so taking it as the
	// single active state would discard that configuration.
	if _, hasRegions := e.graph.CompositeStates[targetState]; !hasRegions {
		e.setCurrentState(targetState)
	}
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

// isSynchronizationTarget reports whether a transition target is a pseudostate
// that replaces the entire active configuration rather than moving one region:
// fork, join, and history.
func isSynchronizationTarget(target ast.Node) bool {
	ps, ok := target.(*ast.PseudostateNode)
	if !ok {
		return false
	}
	switch ps.Kind {
	case ast.PseudostateFork, ast.PseudostateJoin,
		ast.PseudostateShallowHistory, ast.PseudostateDeepHistory:
		return true
	}
	return false
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

// recordHistory returns state's history record, creating it on first use.
func (e *StateExecutor) recordHistory(state *ast.StateNode) *historyRecord {
	record, ok := e.history[state]
	if !ok {
		record = &historyRecord{}
		e.history[state] = record
	}
	return record
}

// fireHistoryTransition takes a transition into a history pseudostate: the
// composite state that owns it is re-entered in the configuration it was last
// left in. Before the state has ever been exited there is nothing to restore, so
// the history's own outgoing transition supplies the default target, exactly as
// UML's default history transition does.
//
// A shallow history restores the substate that was active; a deep history keeps
// descending, restoring the innermost one.
func (e *StateExecutor) fireHistoryTransition(trans *lower.Transition, hist *ast.PseudostateNode) error {
	pass, err := e.passesGuard(trans.Guard)
	if err != nil || !pass {
		return err
	}

	owner, ok := e.graph.PseudostateOwner[hist]
	if !ok || owner == nil {
		return fmt.Errorf("history %s must be declared inside the composite state it restores", hist.Name)
	}

	// The record has to be read before the source configuration is left: a
	// transition out of the owner's own substates would otherwise overwrite it.
	record := e.history[owner]
	if record == nil {
		// Never exited: take the default history transition.
		target, err := e.pseudostateTarget(hist)
		if err != nil {
			return fmt.Errorf("history %s has no default transition and %s has no recorded configuration: %w", hist.Name, owner.Name, err)
		}
		return e.transitionTo(trans, target)
	}

	deep := hist.Kind == ast.PseudostateDeepHistory
	branches := make(map[*ast.StateRegion]*ast.StateNode)

	if len(record.regions) > 0 {
		// The owner keeps its configuration in its regions, so it is re-entered
		// with one branch per region rather than moved to a single state.
		for _, region := range e.graph.CompositeStates[owner] {
			active, recorded := record.regions[region]
			if !recorded {
				continue
			}
			if deep {
				active = e.deepestRecorded(active, branches)
			}
			branches[region] = active
		}
		return e.transitionToInto(trans, owner, branches)
	}

	target := record.child
	if target == nil {
		return fmt.Errorf("history %s: no substate of %s was recorded", hist.Name, owner.Name)
	}
	if deep {
		target = e.deepestRecorded(target, branches)
	}
	return e.transitionToInto(trans, target, branches)
}

// deepestRecorded follows the configuration recorded below state and returns the
// innermost state to enter, adding a branch for every orthogonal region it
// passes through so those regions are restored too. A state whose recorded
// configuration lives in regions is itself the state to enter, since its regions
// carry the rest.
func (e *StateExecutor) deepestRecorded(state *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) *ast.StateNode {
	for {
		record := e.history[state]
		if record == nil {
			return state
		}
		if len(record.regions) > 0 {
			for _, region := range e.graph.CompositeStates[state] {
				if active, recorded := record.regions[region]; recorded {
					branches[region] = e.deepestRecorded(active, branches)
				}
			}
			return state
		}
		if record.child == nil {
			return state
		}
		state = record.child
	}
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

	target, err := e.pseudostateTarget(join)
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
		if !e.isActive(active) {
			continue // Already left as part of an enclosing composite state's teardown.
		}
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

// activeCompositeOwner returns the deepest composite state whose orthogonal
// regions hold the active configuration, or nil when no region is active. It
// walks graph.States rather than the region map so the answer does not depend on
// map iteration order.
func (e *StateExecutor) activeCompositeOwner() *ast.StateNode {
	var deepest *ast.StateNode
	depth := -1
	for _, state := range e.graph.States {
		for _, region := range e.graph.CompositeStates[state] {
			if _, active := e.activeConfig.regionStates[region]; !active {
				continue
			}
			if d := len(e.getParentChain(state)); d > depth {
				deepest, depth = state, d
			}
		}
	}
	return deepest
}

// orderedRegionStates returns the active state of each orthogonal region in
// region declaration order, since exit behaviors run in the order returned.
func (e *StateExecutor) orderedRegionStates() []*ast.StateNode {
	regions := e.orderedActiveRegions()
	states := make([]*ast.StateNode, 0, len(regions))
	for _, region := range regions {
		states = append(states, e.activeConfig.regionStates[region])
	}
	return states
}

// inActiveConfiguration reports whether state is active, either as an active
// state itself or as an ancestor of one.
func (e *StateExecutor) inActiveConfiguration(state *ast.StateNode) bool {
	for _, active := range e.activeStates() {
		for _, ancestor := range e.getParentChain(active) {
			if ancestor == state {
				return true
			}
		}
	}
	return false
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

// maxDoSteps bounds the do activity actions one run may perform, so a machine
// that keeps restarting do behaviors reports instead of spinning forever.
const maxDoSteps = 100000

// RunToCompletion processes queued events until the machine completes or has no
// event or running do behavior left, at which point it suspends. A state's do
// behavior runs while the state is active: each run-to-completion step advances
// every active state's do behavior by one action and then dispatches one event,
// so concurrently active states interleave instead of one running to the end at
// entry, and leaving a state abandons the rest of its do behavior.
func (e *StateExecutor) RunToCompletion() error {
	events, doSteps := 0, 0
	for e.state == StateRunning {
		ran, err := e.runDoRound()
		if err != nil {
			return err
		}
		doSteps += ran
		if doSteps >= maxDoSteps {
			return fmt.Errorf("state machine exceeded max do activity steps (%d), possible non-terminating do behavior", maxDoSteps)
		}
		if e.eventQueue.Len() == 0 && !e.deliverPendingSignal() {
			if ran > 0 {
				continue // do behaviors are still running; they may yet queue events
			}
			e.state = StateSuspended
			return nil
		}
		if events >= maxStateEvents {
			return fmt.Errorf("state machine exceeded max events (%d), possible infinite loop", maxStateEvents)
		}
		events++
		if err := e.processNextEvent(); err != nil {
			return fmt.Errorf("process event: %w", err)
		}
	}
	return nil
}

// startDoActivity registers a state's do behavior as running. Re-entering a
// state restarts its do behavior rather than resuming the abandoned one.
func (e *StateExecutor) startDoActivity(state *ast.StateNode) {
	if len(state.Do) == 0 {
		return
	}
	e.stopDoActivity(state)
	e.doActivities = append(e.doActivities, &doActivity{
		state:   state,
		pending: append([]ast.Node(nil), state.Do...),
	})
}

// stopDoActivity abandons whatever is left of a state's do behavior, which is
// what exiting the state does to it.
func (e *StateExecutor) stopDoActivity(state *ast.StateNode) {
	kept := e.doActivities[:0]
	for _, activity := range e.doActivities {
		if activity.state != state {
			kept = append(kept, activity)
		}
	}
	for i := len(kept); i < len(e.doActivities); i++ {
		e.doActivities[i] = nil
	}
	e.doActivities = kept
}

// runDoRound advances every running do behavior by one action, in the order the
// states were entered, and returns how many actions ran. One round is how
// concurrently active states share the machine: each performs one action before
// any performs its next.
func (e *StateExecutor) runDoRound() (int, error) {
	if len(e.doActivities) == 0 {
		return 0, nil
	}
	round := make([]*doActivity, len(e.doActivities))
	copy(round, e.doActivities)

	ran := 0
	for _, activity := range round {
		if len(activity.pending) == 0 || !e.isRunningDoActivity(activity) {
			continue
		}
		action := activity.pending[0]
		activity.pending = activity.pending[1:]
		if e.trace != nil {
			e.trace.RecordDoStep(activity.state.Name)
		}
		if err := e.executeAction(action); err != nil {
			return ran, fmt.Errorf("do action in state %s: %w", activity.state.Name, err)
		}
		ran++
	}

	// Drop the behaviors that have finished; an activity an action exited the
	// state of is already gone.
	finished := make([]*ast.StateNode, 0, len(e.doActivities))
	kept := e.doActivities[:0]
	for _, activity := range e.doActivities {
		if len(activity.pending) > 0 {
			kept = append(kept, activity)
			continue
		}
		finished = append(finished, activity.state)
	}
	for i := len(kept); i < len(e.doActivities); i++ {
		e.doActivities[i] = nil
	}
	e.doActivities = kept

	// A state completes once its do behavior has finished, which is when its
	// completion transitions become eligible.
	for _, state := range finished {
		if err := e.scheduleCompletionTransitions(state); err != nil {
			return ran, fmt.Errorf("schedule completion of state %s: %w", state.Name, err)
		}
	}
	return ran, nil
}

// isRunningDoActivity reports whether an activity is still registered, which it
// is not once its state has been exited.
func (e *StateExecutor) isRunningDoActivity(activity *doActivity) bool {
	for _, running := range e.doActivities {
		if running == activity {
			return true
		}
	}
	return false
}

// hasRunningDoActivity reports whether a state's do behavior is still running.
func (e *StateExecutor) hasRunningDoActivity(state *ast.StateNode) bool {
	for _, activity := range e.doActivities {
		if activity.state == state {
			return true
		}
	}
	return false
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

// activeStates returns the states currently active, in region declaration
// order: one per region for a composite configuration, otherwise the single
// active state.
func (e *StateExecutor) activeStates() []*ast.StateNode {
	if len(e.activeConfig.regionStates) > 0 {
		return e.orderedRegionStates()
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

	// Enter the initial state of each of the machine's own regions, in declaration
	// order: the order they are entered in is observable.
	if err := e.enterRegionsInto(nil, e.graph.TopRegions, nil); err != nil {
		return err
	}

	// Schedule events for outgoing transitions in all regions
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}

	return nil
}

// descendantChain returns the states from ancestor's child down to leaf,
// outermost first, excluding ancestor itself. A nil ancestor returns the whole
// chain from leaf's outermost ancestor down; leaf alone is returned when
// ancestor is neither nil nor one of its ancestors.
func (e *StateExecutor) descendantChain(ancestor, leaf *ast.StateNode) []*ast.StateNode {
	if leaf == nil {
		return nil
	}
	chain := e.getParentChain(leaf) // leaf .. root
	if ancestor == nil {
		return e.rootToLeaf(leaf)
	}
	for i, state := range chain {
		if state == ancestor {
			descendants := make([]*ast.StateNode, 0, i)
			for j := i - 1; j >= 0; j-- {
				descendants = append(descendants, chain[j])
			}
			return descendants
		}
	}
	return []*ast.StateNode{leaf}
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
		// Entering composite state with orthogonal regions: activate one state per
		// region. Entries for other composite states' regions are left alone, so
		// entering a nested composite does not drop its parent's configuration.
		e.activeConfig.simpleState = nil // Clear simple state

		if err := e.enterRegionsInto(state, regions, branches); err != nil {
			return err
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

	// The do behavior runs while the state is active, interleaved with the do
	// behaviors of the states active alongside it, rather than at entry.
	e.startDoActivity(state)

	// Don't schedule transitions here - let the caller decide when to schedule
	// This prevents double-scheduling in region transitions

	return nil
}

// enterRegionsInto activates one state per orthogonal region of container: the
// state branches names for that region, or the region's initial state. container
// is nil for the machine's own regions, which no state owns.
func (e *StateExecutor) enterRegionsInto(container *ast.StateNode, regions []*ast.StateRegion, branches map[*ast.StateRegion]*ast.StateNode) error {
	for _, region := range regions {
		entry, targeted := branches[region]
		if !targeted {
			entry = e.graph.RegionInitials[region]
		}
		if entry == nil {
			return fmt.Errorf("region %s has no initial state", region.Name)
		}
		e.activeConfig.regionStates[region] = entry
		// Enter the states between the region and its entry, outermost first: a
		// branch may name a state nested below the region, and its ancestors' entry
		// behaviors still have to run. Only descendants of container are entered
		// here — its own ancestors are already active.
		for _, descendant := range e.descendantChain(container, entry) {
			if err := e.enterStateInto(descendant, branches); err != nil {
				return fmt.Errorf("enter starting state in region %s: %w", region.Name, err)
			}
		}
	}
	return nil
}

// exitState executes exit behaviors when leaving a state.
func (e *StateExecutor) exitState(state *ast.StateNode) error {
	if state == nil {
		return nil
	}

	// Check if this state is a composite state with regions
	regions, isComposite := e.graph.CompositeStates[state]

	// The active configuration of every composite state lives in one map, so only
	// this state's own regions may be recorded or torn down here: touching the
	// whole map would stop the regions of an enclosing composite state too.
	active := make(map[*ast.StateRegion]*ast.StateNode, len(regions))
	for _, region := range regions {
		if regionState, isActive := e.activeConfig.regionStates[region]; isActive {
			active[region] = regionState
		}
	}

	// Remember the configuration being left, so a history pseudostate owned by
	// this state or by its parent can restore it.
	if len(active) > 0 {
		e.recordHistory(state).regions = active
	}
	if parent := e.graph.ParentState[state]; parent != nil {
		e.recordHistory(parent).child = state
	}

	// Exit the active state of each of this state's regions, in declaration order.
	if isComposite {
		for _, region := range regions {
			regionState, isActive := active[region]
			if !isActive {
				continue
			}
			// Clear the entry first: the recursive exit walks the same map, and the
			// region still pointing at regionState would exit it a second time.
			delete(e.activeConfig.regionStates, region)
			// A region's active state may be nested below the region, so exit it and
			// the states between it and this one: their exit behaviors run too.
			for current := regionState; current != nil && current != state; current = e.graph.ParentState[current] {
				if err := e.exitState(current); err != nil {
					return fmt.Errorf("exit region state: %w", err)
				}
			}
		}
	}

	// Leaving the state abandons whatever is left of its do behavior, before the
	// exit behavior runs.
	e.stopDoActivity(state)

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
		inv, ok := nestedInvocation(node)
		if !ok {
			return fmt.Errorf("state action %s performs no action", stateActionName(node))
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

// stateActionName names a state action in diagnostics, falling back to what it
// references when the usage is anonymous (`entry a.b;`).
func stateActionName(u *ast.Usage) string {
	if u.Ident.Name != "" {
		return u.Ident.Name
	}
	for _, rel := range u.Relationships {
		if rel.Kind != ast.RelReferences && rel.Kind != ast.RelTyping {
			continue
		}
		switch target := rel.Target.(type) {
		case *ast.QualifiedName:
			return qualifiedNameText(target)
		case *ast.FeatureChainExpr:
			return "feature chain " + qualifiedNameText(target.Member)
		}
	}
	return "<anonymous>"
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

// ActiveStates returns the machine's active state configuration: the single
// active state, or one state per orthogonal region, in declaration order.
func (e *StateExecutor) ActiveStates() []*ast.StateNode {
	return e.activeStates()
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
// It is the same step RunToCompletion repeats: every active state's do behavior
// advances by one action, then the next event is dispatched. Advancing the do
// behaviors is progress in itself, so a step that ran one and found no event to
// dispatch succeeds — the completion transition it enables is queued next.
func (e *StateExecutor) ProcessNextEvent() error {
	ran, err := e.runDoRound()
	if err != nil {
		return err
	}
	if e.eventQueue.Len() == 0 && ran > 0 {
		return nil
	}
	return e.processNextEvent()
}

// HasPendingWork reports whether stepping the machine can still make progress:
// an event is queued, or a state's do behavior has actions left to run.
func (e *StateExecutor) HasPendingWork() bool {
	return e.eventQueue.Len() > 0 || len(e.doActivities) > 0
}

// RunDoRound advances every active state's do behavior by one action, without
// dispatching any event, and reports how many actions ran.
func (e *StateExecutor) RunDoRound() (int, error) {
	return e.runDoRound()
}

// HasPendingDoWork reports whether some active state's do behavior still has an
// action to run. Such work is due now, unlike a queued event's timestamp.
func (e *StateExecutor) HasPendingDoWork() bool {
	for _, activity := range e.doActivities {
		if len(activity.pending) > 0 {
			return true
		}
	}
	return false
}
