package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ActionExecutor executes action bodies using token-flow semantics.
type ActionExecutor struct {
	ctx         *Context
	action      *symbols.Symbol
	graph       *lower.ActionGraph // Execution IR
	tokens      []Token
	state       ExecutionState
	nextTokenID int64
	stepCount   int // Current step number for tracing
	breakpoints map[string]bool
	// firedBreakpoints records the token visits a breakpoint already stopped on.
	firedBreakpoints map[breakpointVisit]bool
	results          map[string]Value  // Accumulated results from consumed final tokens
	mergeVisited     map[ast.Node]bool // Track merge node visits
	inputs           map[string]Value  // Input parameter bindings seeded into the initial token
	pausedAt         string            // Node name RunToCompletion stopped at, empty when it ran to the end
}

// breakpointVisit identifies one token's stay at one node.
type breakpointVisit struct {
	token int64
	node  ast.Node
}

// SetInputs binds input parameter values that seed the initial token's data.
// Inputs are applied after attribute defaults, so they override defaults with
// the same name. Must be called before initialize().
func (e *ActionExecutor) SetInputs(inputs map[string]Value) {
	e.inputs = inputs
}

// newActionExecutor creates an action executor.
func newActionExecutor(ctx *Context, action *symbols.Symbol) (*ActionExecutor, error) {
	if action.Kind != symbols.SymbolActionUsage && action.Kind != symbols.SymbolActionDef {
		return nil, fmt.Errorf("symbol %s is not an action", action.Name)
	}

	// Lower AST to execution graph
	graph, err := lower.ToActionGraph(action.Decl)
	if err != nil {
		return nil, fmt.Errorf("lower action graph: %w", err)
	}

	exec := &ActionExecutor{
		ctx:         ctx,
		action:      action,
		graph:       graph,
		tokens:      make([]Token, 0),
		state:       StateReady,
		nextTokenID: 1,
		breakpoints: make(map[string]bool),

		firedBreakpoints: make(map[breakpointVisit]bool),
		results:          make(map[string]Value),
		mergeVisited:     make(map[ast.Node]bool),
	}

	return exec, nil
}

// Step advances execution by one step for all active tokens.
// Safely handles token slice modifications (fork/join) by collecting indices first.
//
// A token that reaches an accept with no message it can consume parks there
// rather than failing: the action is suspended until a matching message
// arrives. When a step moves nothing and at least one token is parked, the
// executor enters StateWaiting instead of reporting a deadlock — a caller
// driving Step itself (the REPL, or a state machine running in the same
// context) may still post the awaited message and step again, which resumes
// the parked token. RunToCompletion has no such caller, so it turns a step
// that leaves the executor waiting into ErrAcceptDeadlock.
//
// Returns an error if a deadlock unrelated to accepts is detected (no progress
// made and nothing is waiting for a message).
func (e *ActionExecutor) Step() error {
	if e.state == StateCompleted {
		return nil // Already completed
	}

	if e.state == StateReady {
		return fmt.Errorf("executor not initialized (call initialize first)")
	}

	// Stepping resumes a run a breakpoint suspended.
	if e.state == StateSuspended {
		e.state = StateRunning
		e.pausedAt = ""
	}

	// A waiting executor is asked again whether its parked tokens can proceed:
	// messages may have been posted since the step that parked them.
	if e.state == StateWaiting {
		e.state = StateRunning
	}

	// Snapshot token state before step (for deadlock detection)
	tokenCountBefore := len(e.tokens)
	tokenLocationsBefore := make([]ast.Node, len(e.tokens))
	for i, t := range e.tokens {
		tokenLocationsBefore[i] = t.Location
	}

	// Collect token indices to step (snapshot before iteration)
	tokenIndices := make([]int, len(e.tokens))
	for i := range e.tokens {
		tokenIndices[i] = i
	}

	// Step tokens in reverse order to handle removal safely
	// (removing token at higher index doesn't affect lower indices)
	for i := len(tokenIndices) - 1; i >= 0; i-- {
		// Check if token still exists (may have been removed by join/final)
		if i >= len(e.tokens) {
			continue
		}

		err := e.stepToken(i)
		if err != nil {
			return err
		}
	}

	// Deadlock detection: check if any progress was made
	progressMade := false

	// Progress indicators:
	// 1. Token count changed (fork/join/final consumed/created tokens)
	if len(e.tokens) != tokenCountBefore {
		progressMade = true
	}

	// 2. At least one token moved to different location
	if !progressMade && len(e.tokens) > 0 {
		for i := 0; i < len(e.tokens) && i < len(tokenLocationsBefore); i++ {
			if e.tokens[i].Location != tokenLocationsBefore[i] {
				progressMade = true
				break
			}
		}
	}

	// 3. All tokens consumed (completion)
	if len(e.tokens) == 0 {
		progressMade = true
	}

	// If no progress and tokens remain, either the action is suspended waiting
	// for a message, or it is stuck for a reason no message can resolve.
	if !progressMade && len(e.tokens) > 0 {
		if !e.anyTokenWaiting() {
			return fmt.Errorf("deadlock detected: %d token(s) stuck, no progress made", len(e.tokens))
		}
		e.state = StateWaiting
	}

	// Increment step count
	e.stepCount++

	// Record trace after step completes
	if e.trace() != nil {
		e.trace().RecordActionStep(e.stepCount, e.tokens)
	}

	return nil
}

// anyTokenWaiting reports whether some token is parked at an accept.
func (e *ActionExecutor) anyTokenWaiting() bool {
	for _, token := range e.tokens {
		if token.Wait != nil {
			return true
		}
	}
	return false
}

// waitingTokens returns the parked tokens, in token-ID order, so that a report
// of what an action is waiting for does not depend on step scheduling.
func (e *ActionExecutor) waitingTokens() []Token {
	waiting := make([]Token, 0, len(e.tokens))
	for _, token := range e.tokens {
		if token.Wait != nil {
			waiting = append(waiting, token)
		}
	}
	sort.Slice(waiting, func(i, j int) bool { return waiting[i].ID < waiting[j].ID })
	return waiting
}

// deadlockError describes a suspension that can never end: the accepts still
// waiting, and any token blocked for another reason alongside them.
func (e *ActionExecutor) deadlockError() error {
	waiting := e.waitingTokens()
	descriptions := make([]string, 0, len(waiting))
	for _, token := range waiting {
		descriptions = append(descriptions, token.Wait.String())
	}
	if blocked := len(e.tokens) - len(waiting); blocked > 0 {
		descriptions = append(descriptions,
			fmt.Sprintf("%d token(s) blocked for another reason", blocked))
	}
	return fmt.Errorf("%w in action %s: nothing can post the awaited message (%s)",
		ErrAcceptDeadlock, e.action.Name, strings.Join(descriptions, "; "))
}

// RunToCompletion executes until StateCompleted, a breakpoint, or error.
// Includes infinite loop protection.
//
// A run stops as soon as a token sits on a node a breakpoint was set on
// (see SetBreakpoint), leaving the tokens where they are so the run can be
// resumed by calling RunToCompletion again or stepped with Step; PausedAt names
// the node it stopped at. With no breakpoints set the run is unconditional.
//
// Nothing outside the action can post a message while this runs, so an action
// whose every remaining token is parked at an accept can never be resumed: the
// suspension is a deadlock and is reported as ErrAcceptDeadlock at the first
// step that makes no progress. A parked action therefore cannot spend the step
// budget spinning — the budget is only consumed by steps that move something.
func (e *ActionExecutor) RunToCompletion() error {
	const maxSteps = 10000
	steps := 0

	e.pausedAt = ""
	if e.state == StateSuspended {
		e.state = StateRunning
	}

	// A run may start from StateWaiting: a caller that stepped an action into a
	// suspension and then posted the awaited message resumes it here.
	for e.state == StateRunning || e.state == StateWaiting {
		if node := e.breakpointHit(); node != "" {
			e.pausedAt = node
			e.state = StateSuspended
			return nil
		}

		if steps >= maxSteps {
			return fmt.Errorf("execution exceeded max steps (%d), possible infinite loop", maxSteps)
		}

		if err := e.Step(); err != nil {
			return err
		}
		if e.state == StateWaiting {
			return e.deadlockError()
		}

		steps++
	}

	return nil
}

// breakpointHit returns the name of a breakpoint node a token sits on and has
// not yet stopped the run at, or "" if none does. Firing once per token and
// visit means a resumed run continues past the node it stopped at, while a
// token that leaves and comes back around a loop stops again.
func (e *ActionExecutor) breakpointHit() string {
	if len(e.breakpoints) == 0 {
		return ""
	}
	for visit := range e.firedBreakpoints {
		if loc, ok := e.tokenLocation(visit.token); !ok || loc != visit.node {
			delete(e.firedBreakpoints, visit)
		}
	}
	for _, token := range e.tokens {
		name := ActionNodeName(token.Location)
		if name == "" || !e.breakpoints[name] {
			continue
		}
		visit := breakpointVisit{token: token.ID, node: token.Location}
		if e.firedBreakpoints[visit] {
			continue
		}
		if e.firedBreakpoints == nil {
			e.firedBreakpoints = make(map[breakpointVisit]bool)
		}
		e.firedBreakpoints[visit] = true
		return name
	}
	return ""
}

// tokenLocation returns where the given token sits, if it is still active.
func (e *ActionExecutor) tokenLocation(id int64) (ast.Node, bool) {
	for _, token := range e.tokens {
		if token.ID == id {
			return token.Location, true
		}
	}
	return nil, false
}

// PausedAt returns the breakpoint node the last run stopped at, or "" when the
// run was not stopped by a breakpoint.
func (e *ActionExecutor) PausedAt() string {
	return e.pausedAt
}

// ActionNodeName returns the declared name of an action graph node, or "" when
// the node is anonymous or not a named node kind.
func ActionNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return n.Name
	case *ast.FinalNode:
		return n.Name
	case *ast.ForkNode:
		return n.Name
	case *ast.JoinNode:
		return n.Name
	case *ast.MergeNode:
		return n.Name
	case *ast.DecisionNode:
		return n.Name
	case *ast.ActionExecutionNode:
		return n.Name
	case *ast.StateNode:
		return n.Name
	case *ast.Usage:
		if n.Ident.Name != "" {
			return n.Ident.Name
		}
		return n.Ident.ShortName
	case *ast.Definition:
		if n.Ident.Name != "" {
			return n.Ident.Name
		}
		return n.Ident.ShortName
	default:
		return ""
	}
}

// NodeNames returns the declared names of the action's graph nodes, in
// declaration order. Anonymous nodes are omitted; a debugger uses it to check
// that a breakpoint names a node that exists.
func (e *ActionExecutor) NodeNames() []string {
	names := make([]string, 0, len(e.graph.Nodes))
	for _, node := range e.graph.Nodes {
		if name := ActionNodeName(node); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// extractGraph builds node and edge maps from action AST.

// initializeAttributes populates tokenData with attribute default values from action.
func (e *ActionExecutor) initializeAttributes(tokenData map[string]Value) error {
	// Get action node
	actionNode, ok := e.action.Decl.(*ast.Usage)
	if !ok {
		actionDef, ok := e.action.Decl.(*ast.Definition)
		if !ok {
			return fmt.Errorf("action symbol has invalid node type")
		}
		actionNode = &ast.Usage{Members: actionDef.Members}
	}

	// Extract attribute defaults
	for _, member := range actionNode.Members {
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
				tokenData[usage.Ident.Name] = value
			}
		}
	}

	return nil
}

// initialize spawns initial token at InitialNode.
func (e *ActionExecutor) initialize() error {
	// Use initial node from graph
	if e.graph.Initial == nil {
		return fmt.Errorf("no initial node found in action %s", e.action.Name)
	}

	initialNode := e.graph.Initial

	// Spawn initial token
	tokenData := make(map[string]Value)

	// Initialize with action attribute defaults
	if err := e.initializeAttributes(tokenData); err != nil {
		return fmt.Errorf("initialize attributes: %w", err)
	}

	// Apply input parameter bindings, overriding any defaults with the same name.
	for name, value := range e.inputs {
		tokenData[name] = value
	}

	token := Token{
		ID:       e.nextTokenID,
		Location: initialNode,
		Data:     tokenData,
	}
	e.nextTokenID++
	e.tokens = append(e.tokens, token)

	e.state = StateRunning
	return nil
}

// stepToken advances a specific token by index.
func (e *ActionExecutor) stepToken(tokenIdx int) error {
	if tokenIdx < 0 || tokenIdx >= len(e.tokens) {
		return fmt.Errorf("invalid token index %d", tokenIdx)
	}

	token := &e.tokens[tokenIdx]

	switch node := token.Location.(type) {
	case *ast.InitialNode:
		return e.stepInitialNode(tokenIdx)
	case *ast.FinalNode:
		return e.stepFinalNode(tokenIdx)
	case *ast.ForkNode:
		return e.stepForkNode(tokenIdx)
	case *ast.JoinNode:
		return e.stepJoinNode(tokenIdx)
	case *ast.MergeNode:
		return e.stepMergeNode(tokenIdx)
	case *ast.DecisionNode:
		return e.stepDecisionNode(tokenIdx)
	case *ast.ActionExecutionNode:
		return e.stepActionExecutionNode(tokenIdx)
	case *ast.Usage:
		// Nested action invocation
		if node.Kind == ast.UsageAction {
			return e.stepNestedAction(tokenIdx)
		}
		return fmt.Errorf("unsupported usage kind in action: %v", node.Kind)
	default:
		return fmt.Errorf("unsupported node type: %T", node)
	}
}

// stepInitialNode advances token from initial node to successors.
func (e *ActionExecutor) stepInitialNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	successors := e.graph.Edges[token.Location]

	if len(successors) == 0 {
		return fmt.Errorf("initial node has no successors")
	}

	// Move token to first successor (initial should have exactly 1)
	token.Location = successors[0]
	return nil
}

// stepFinalNode consumes token and checks for completion.
func (e *ActionExecutor) stepFinalNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]

	// Save token data to results before consuming
	for k, v := range token.Data {
		e.results[k] = v
	}

	// Remove token
	e.tokens = append(e.tokens[:tokenIdx], e.tokens[tokenIdx+1:]...)

	// Check if all tokens consumed
	if len(e.tokens) == 0 {
		e.state = StateCompleted
	}

	return nil
}

// stepForkNode spawns N tokens (one per successor).
func (e *ActionExecutor) stepForkNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node := token.Location.(*ast.ForkNode)

	successors := e.graph.Edges[node]
	if len(successors) == 0 {
		return fmt.Errorf("fork node %s has no successors", node.Name)
	}

	// Create N tokens (one per successor)
	newTokens := make([]Token, 0, len(successors))
	for _, succ := range successors {
		newToken := Token{
			ID:       e.nextTokenID,
			Location: succ,
			Data:     copyTokenData(token.Data), // Copy data to each fork
		}
		e.nextTokenID++
		newTokens = append(newTokens, newToken)
	}

	// Remove original token, add new tokens
	e.tokens = append(e.tokens[:tokenIdx], e.tokens[tokenIdx+1:]...)
	e.tokens = append(e.tokens, newTokens...)

	return nil
}

// stepJoinNode synchronizes tokens from all incoming edges.
// Waits for tokens on ALL incoming edges before firing.
func (e *ActionExecutor) stepJoinNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node := token.Location.(*ast.JoinNode)

	// Get incoming edges
	incomingEdges := e.getIncomingEdges(node)

	// Count tokens at this join node
	tokensAtJoin := 0
	for _, t := range e.tokens {
		if t.Location == node {
			tokensAtJoin++
		}
	}

	// Wait until all incoming edges have tokens
	if tokensAtJoin < len(incomingEdges) {
		// Not ready yet - barrier synchronization requires ALL incoming tokens.
		// Returns nil (no-op) until all tokens arrive. Deadlock detection handled separately (Task 11).
		return nil
	}

	// Ready: collect all join tokens and remaining tokens
	joinTokens := make([]Token, 0, tokensAtJoin)
	remainingTokens := make([]Token, 0, len(e.tokens)-tokensAtJoin)

	for _, t := range e.tokens {
		if t.Location == node {
			joinTokens = append(joinTokens, t)
		} else {
			remainingTokens = append(remainingTokens, t)
		}
	}

	// Merge token data (last-write-wins)
	mergedData := make(map[string]Value)
	for _, t := range joinTokens {
		for k, v := range t.Data {
			mergedData[k] = v
		}
	}

	// Get successor
	successors := e.graph.Edges[node]
	if len(successors) == 0 {
		return fmt.Errorf("join node %s has no successors", node.Name)
	}
	if len(successors) > 1 {
		return fmt.Errorf("join node %s has multiple successors", node.Name)
	}

	// Create output token at successor
	outputToken := Token{
		ID:       e.nextTokenID,
		Location: successors[0],
		Data:     mergedData,
	}
	e.nextTokenID++

	// Replace tokens: remove join tokens, add output token
	e.tokens = append(remainingTokens, outputToken)

	return nil
}

// getIncomingEdges finds all nodes that have edges targeting the given node.
func (e *ActionExecutor) getIncomingEdges(node ast.Node) []ast.Node {
	incoming := make([]ast.Node, 0)
	for source, targets := range e.graph.Edges {
		for _, target := range targets {
			if target == node {
				incoming = append(incoming, source)
				break // Only count each source once
			}
		}
	}
	return incoming
}

// stepMergeNode implements OR-join semantics (first-token-wins).
func (e *ActionExecutor) stepMergeNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	mergeNode, ok := token.Location.(*ast.MergeNode)
	if !ok {
		return fmt.Errorf("expected MergeNode, got %T", token.Location)
	}

	// Check if merge already visited
	if e.mergeVisited[mergeNode] {
		// Discard token (first-wins)
		e.tokens = append(e.tokens[:tokenIdx], e.tokens[tokenIdx+1:]...)
		return nil
	}

	// Mark merge visited, pass token through
	e.mergeVisited[mergeNode] = true

	successors := e.graph.Edges[mergeNode]
	if len(successors) == 0 {
		return fmt.Errorf("merge node %s has no successors", mergeNode.Name)
	}
	if len(successors) > 1 {
		return fmt.Errorf("merge node %s has multiple successors (not yet supported)", mergeNode.Name)
	}

	token.Location = successors[0]
	return nil
}

// stepDecisionNode evaluates guards and routes token to matching branch.
func (e *ActionExecutor) stepDecisionNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	decisionNode, ok := token.Location.(*ast.DecisionNode)
	if !ok {
		return fmt.Errorf("expected DecisionNode, got %T", token.Location)
	}

	// Get successors (outgoing edges from decision)
	successors := e.graph.Edges[decisionNode]
	if len(successors) == 0 {
		return fmt.Errorf("decision node %s has no successors", decisionNode.Name)
	}

	// Evaluate guards for each successor
	ec := NewEvalContext(e.ctx, nil)
	ec.Push(token.Data) // Make token data available to guard expressions

	// Two-pass evaluation:
	// 1. Evaluate all guarded edges first
	// 2. If none match, use unguarded edge as fallback (else branch)

	var unguardedEdge ast.Node

	// Pass 1: Check guarded edges
	for _, succ := range successors {
		// Get guard for this edge (if any)
		var guard ast.Node
		if guards, ok := e.graph.Guards[decisionNode]; ok {
			guard = guards[succ]
		}

		// No guard = remember for fallback
		if guard == nil {
			unguardedEdge = succ
			continue
		}

		// Evaluate guard
		result, err := ec.Eval(guard)
		if err != nil {
			return fmt.Errorf("eval guard: %w", err)
		}

		// Guard must be boolean
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			return fmt.Errorf("decision node %s: guard must evaluate to boolean, got %v", decisionNode.Name, result.Kind)
		}

		// Check if guard is true
		if result.Const.Bool {
			token.Location = succ
			return nil
		}
	}

	// Pass 2: Use unguarded edge as fallback
	if unguardedEdge != nil {
		token.Location = unguardedEdge
		return nil
	}

	return fmt.Errorf("decision node %s: no true guard", decisionNode.Name)
}

// copyTokenData creates a shallow copy of token data map.
// This is sufficient as Value structs are copied by value, and pointer
// fields (Sequence, Set) are intended to be shared across forked tokens.
func copyTokenData(data map[string]Value) map[string]Value {
	copy := make(map[string]Value)
	for k, v := range data {
		copy[k] = v
	}
	return copy
}

// stepActionExecutionNode evaluates inline expression or invokes nested action.
func (e *ActionExecutor) stepActionExecutionNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node, ok := token.Location.(*ast.ActionExecutionNode)
	if !ok {
		return fmt.Errorf("expected ActionExecutionNode, got %T", token.Location)
	}

	if node.Expression != nil {
		// Evaluate inline expression
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(token.Data) // Make token data available
		result, err := ec.Eval(node.Expression)
		if err != nil {
			return fmt.Errorf("eval expression: %w", err)
		}

		// Store result: check if dataFlows specify output pin, else use "result"
		outputPin := "result"
		if flows, ok := e.graph.DataFlows[node]; ok && len(flows) > 0 {
			// Use source pin from first data flow as output pin
			if flows[0].SourcePin != "" {
				outputPin = flows[0].SourcePin
			}
		}
		token.Data[outputPin] = result
	} else if node.ActionRef != nil {
		outputs, err := invokeAction(
			e.ctx, e.action.Scope, actionInvocation{target: node.ActionRef}, token.Data,
		)
		if err != nil {
			return err
		}
		for name, value := range outputs {
			token.Data[name] = value
		}
	}

	// Advance to successor
	successors := e.graph.Edges[token.Location]
	if len(successors) == 0 {
		return fmt.Errorf("action node %s has no successors", node.Name)
	}
	if len(successors) > 1 {
		return fmt.Errorf("action node %s has multiple successors (decision nodes not yet supported)", node.Name)
	}

	// Apply data flows: transfer data from this node's output pins to target input pins
	e.applyDataFlows(token, node)

	token.Location = successors[0]
	return nil
}

// stepNestedAction executes a nested action usage.
func (e *ActionExecutor) stepNestedAction(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	usage, ok := token.Location.(*ast.Usage)
	if !ok {
		return fmt.Errorf("expected Usage, got %T", token.Location)
	}

	// An accept node waits for a message of its parameter's type. Until one
	// arrives the token parks here: the action is suspended, not failed, and
	// the next step retries the match.
	accept, isAccept := e.graph.Accepts[usage]
	if isAccept {
		msg, taken := e.ctx.TakeMessage(func(m Message) bool {
			if !m.arrivedAt(accept.ViaPort) || !m.carriesSignal(accept.SignalType) {
				return false
			}
			// A message routed to this accept's port is already addressed by the
			// connection it travelled over, so the accept's own name does not
			// have to appear in it.
			return accept.ViaPort != "" || m.addressedTo(usage.Ident.Name)
		})
		if !taken {
			if token.Wait == nil {
				token.Wait = &AcceptWait{
					ParamName:  accept.ParamName,
					SignalType: accept.SignalType,
					ViaPort:    accept.ViaPort,
					// stepCount is incremented once the step finishes, so the
					// step now in progress is the next one.
					Since: e.stepCount + 1,
				}
			}
			return nil
		}
		token.Wait = nil
		token.Data[accept.ParamName] = msg.Payload["value"]
	}

	// A usage that performs another action (perform X / action a : X / a = X(...))
	// runs that action to completion before its own body.
	if inv, ok := nestedInvocation(usage); ok {
		outputs, err := invokeAction(e.ctx, e.action.Scope, inv, token.Data)
		if err != nil {
			return err
		}
		for name, value := range outputs {
			token.Data[name] = value
		}
	}

	// Execute the node's lowered statements in declaration order.
	for _, stmt := range e.graph.Bodies[usage] {
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(token.Data) // Token data available for evaluation

		switch s := stmt.(type) {
		case lower.Send:
			msg, err := ec.buildMessage(e.action.Scope, s)
			if err != nil {
				return err
			}
			e.ctx.post(e.graph.Connections, msg, s)
		case lower.Assign:
			if s.Target == "" {
				return fmt.Errorf("nested action %s: unsupported assignment target", usage.Ident.Name)
			}
			value, err := ec.Eval(s.Value)
			if err != nil {
				return fmt.Errorf("eval assignment RHS: %w", err)
			}
			token.Data[s.Target] = value
		default:
			return fmt.Errorf("nested action %s: unsupported statement %T", usage.Ident.Name, stmt)
		}
	}

	// Advance to successor
	successors := e.graph.Edges[token.Location]
	if len(successors) == 0 {
		return fmt.Errorf("nested action %s has no successors", usage.Ident.Name)
	}
	if len(successors) > 1 {
		return fmt.Errorf("nested action %s has multiple successors", usage.Ident.Name)
	}

	token.Location = successors[0]
	return nil
}

// applyDataFlows transfers data along object flow edges.
// Copies data from source pins to target pins for all outgoing data flows.
func (e *ActionExecutor) applyDataFlows(token *Token, sourceNode ast.Node) {
	flows, ok := e.graph.DataFlows[sourceNode]
	if !ok || len(flows) == 0 {
		return
	}

	for _, flow := range flows {
		// Get data from source pin
		sourceData, ok := token.Data[flow.SourcePin]
		if !ok {
			// No data at source pin - skip this flow
			continue
		}

		// Store in target pin (will be available when token reaches target)
		token.Data[flow.TargetPin] = sourceData
	}
}

// --- Public accessor methods for REPL debugging ---

// Tokens returns a copy of active tokens.
func (e *ActionExecutor) Tokens() []Token {
	tokens := make([]Token, len(e.tokens))
	copy(tokens, e.tokens)
	return tokens
}

// State returns current execution state.
func (e *ActionExecutor) State() ExecutionState {
	return e.state
}

// Results returns accumulated results from final nodes.
func (e *ActionExecutor) Results() map[string]Value {
	return e.results
}

// SetBreakpoint adds a breakpoint at the given node name.
func (e *ActionExecutor) SetBreakpoint(nodeName string) {
	e.breakpoints[nodeName] = true
}

// ClearBreakpoints removes all breakpoints.
func (e *ActionExecutor) ClearBreakpoints() {
	e.breakpoints = make(map[string]bool)
	e.firedBreakpoints = make(map[breakpointVisit]bool)
}

// trace returns the recorder this executor's context is attached to, so turning
// reporting on or off reaches an execution already under way.
func (e *ActionExecutor) trace() *TraceRecorder {
	return e.ctx.trace
}

// SetTrace sets the trace recorder for this executor and the context it
// evaluates in.
func (e *ActionExecutor) SetTrace(trace *TraceRecorder) {
	e.ctx.SetTrace(trace)
}

// ActionSymbol returns the action being executed.
func (e *ActionExecutor) ActionSymbol() *symbols.Symbol {
	return e.action
}
