package runtime

import (
	"fmt"

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
	stepCount   int              // Current step number for tracing
	breakpoints map[string]bool
	results     map[string]Value // Accumulated results from consumed final tokens
	trace       *TraceRecorder   // Optional trace recorder for testing
	messageQueue []Message       // Queue of sent messages (FIFO)
	mergeVisited map[ast.Node]bool // Track merge node visits
}

// Message represents a signal instance sent via send action.
type Message struct {
	SignalType string           // Signal type name
	Payload    map[string]Value // Signal attribute values
}

type objectFlow struct {
	SourcePin string
	TargetPin string
	Target    ast.Node
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
		ctx:          ctx,
		action:       action,
		graph:        graph,
		tokens:       make([]Token, 0),
		state:        StateReady,
		nextTokenID:  1,
		breakpoints:  make(map[string]bool),
		results:      make(map[string]Value),
		mergeVisited: make(map[ast.Node]bool),
	}
	
	return exec, nil
}

// Step advances execution by one step for all active tokens.
// Safely handles token slice modifications (fork/join) by collecting indices first.
// Returns error if deadlock detected (no progress made).
func (e *ActionExecutor) Step() error {
	if e.state == StateCompleted {
		return nil // Already completed
	}
	
	if e.state == StateReady {
		return fmt.Errorf("executor not initialized (call initialize first)")
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
	
	// If no progress and tokens remain, deadlock detected
	if !progressMade && len(e.tokens) > 0 {
		return fmt.Errorf("deadlock detected: %d token(s) stuck, no progress made", len(e.tokens))
	}
	
	// Increment step count
	e.stepCount++
	
	// Record trace after step completes
	if e.trace != nil {
		e.trace.RecordActionStep(e.stepCount, e.tokens)
	}
	
	return nil
}

// RunToCompletion executes until StateCompleted or error.
// Includes infinite loop protection.
func (e *ActionExecutor) RunToCompletion() error {
	const maxSteps = 10000
	steps := 0
	
	for e.state == StateRunning {
		if steps >= maxSteps {
			return fmt.Errorf("execution exceeded max steps (%d), possible infinite loop", maxSteps)
		}
		
		err := e.Step()
		if err != nil {
			return err
		}
		
		steps++
	}
	
	return nil
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
		return fmt.Errorf("nested action invocation not yet implemented")
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
	
	// Check for accept parameters and consume messages
	for _, member := range usage.Members {
		actualMember := member
		if membership, ok := member.(*ast.Membership); ok {
			actualMember = membership.Member
		}
		
		if paramUsage, ok := actualMember.(*ast.Usage); ok && paramUsage.IsAccept {
			// Accept action: consume message from queue
			if len(e.messageQueue) == 0 {
				return fmt.Errorf("accept action %s: no messages in queue", paramUsage.Ident.Name)
			}
			
			// Dequeue first message (FIFO)
			msg := e.messageQueue[0]
			e.messageQueue = e.messageQueue[1:]
			
			// Bind message to parameter name in token data
			paramName := paramUsage.Ident.Name
			token.Data[paramName] = msg.Payload["value"]
		}
	}
	
	// Execute nested action members (assignments, send statements, expressions)
	for _, member := range usage.Members {
		// Unwrap Membership if present
		actualMember := member
		if membership, ok := member.(*ast.Membership); ok {
			actualMember = membership.Member
		}
		
		// Execute send statements
		if send, ok := actualMember.(*ast.SendStatement); ok {
			ec := NewEvalContext(e.ctx, nil)
			ec.Push(token.Data) // Token data available for evaluation
			
			// Evaluate message expression
			msgValue, err := ec.Eval(send.Message)
			if err != nil {
				return fmt.Errorf("eval send message: %w", err)
			}
			
			// Queue message (simple FIFO queue)
			msg := Message{
				SignalType: "GenericSignal", // TODO: extract from message type
				Payload:    map[string]Value{"value": msgValue},
			}
			e.messageQueue = append(e.messageQueue, msg)
			continue
		}
		
		// Execute assignment actions
		if assign, ok := actualMember.(*ast.AssignmentActionNode); ok {
			ec := NewEvalContext(e.ctx, nil)
			ec.Push(token.Data) // Token data available for RHS evaluation
			
			// Evaluate RHS
			value, err := ec.Eval(assign.Value)
			if err != nil {
				return fmt.Errorf("eval assignment RHS: %w", err)
			}
			
			// Store in token data (updates shared state)
			// Target can be QualifiedName or FeatureReference
			var targetName string
			if qname, ok := assign.Target.(*ast.QualifiedName); ok && len(qname.Parts) > 0 {
				targetName = qname.Parts[len(qname.Parts)-1].Text
			} else if fref, ok := assign.Target.(*ast.FeatureReference); ok && fref.Name != nil && len(fref.Name.Parts) > 0 {
				targetName = fref.Name.Parts[len(fref.Name.Parts)-1].Text
			}
			
			if targetName != "" {
				token.Data[targetName] = value
			}
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
}

// SetTrace sets the trace recorder for this executor.
func (e *ActionExecutor) SetTrace(trace *TraceRecorder) {
	e.trace = trace
}

// ActionSymbol returns the action being executed.
func (e *ActionExecutor) ActionSymbol() *symbols.Symbol {
	return e.action
}
