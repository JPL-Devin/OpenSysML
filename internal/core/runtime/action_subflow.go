package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// An action node owning a flow runs it as subperformances (`subactions :> subperformances`):
// a token moves into the node's performance, and the node completes when its last token retires.

// graphOf returns the flow a performance runs.
func (e *ActionExecutor) graphOf(frame *actionFrame) *lower.ActionGraph {
	return frame.graph
}

// tokenGraph returns the flow the token at this index is running in.
func (e *ActionExecutor) tokenGraph(tokenIdx int) *lower.ActionGraph {
	return e.graphOf(e.tokens[tokenIdx].frame)
}

// subflowOf returns the flow a node owns, and whether it owns one.
func (e *ActionExecutor) subflowOf(graph *lower.ActionGraph, node ast.Node) (*lower.Subflow, bool) {
	sub, owns := graph.Subflows[node]
	return sub, owns && sub != nil
}

// enterSubflow moves a token into the flow its node owns, run by the node's performance;
// the flow was validated at initialize(), so an unbuildable one is an error here.
func (e *ActionExecutor) enterSubflow(tokenIdx int, perf *actionFrame) error {
	token := &e.tokens[tokenIdx]
	node := token.Location
	if perf.graph == nil || perf.graph.Initial == nil {
		return fmt.Errorf("%w: action node %s owns a flow that cannot be built",
			ErrInvalidActionFlow, ActionNodeName(node))
	}
	token.frame = perf
	token.Location = perf.graph.Initial
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeEnter(ActionNodeName(node))
	}
	return nil
}

// runSubflow performs the flow perf owns to completion where a body statement,
// not a token of the enclosing flow, performs its node: the tokens of that flow
// alone are stepped until its last one retires. Nothing outside can post a message
// meanwhile, so a token parked at an accept is a deadlock, as under RunToCompletion.
func (e *ActionExecutor) runSubflow(perf *actionFrame) error {
	node := perf.node
	if perf.graph == nil || perf.graph.Initial == nil {
		return fmt.Errorf("%w: action node %s owns a flow that cannot be built",
			ErrInvalidActionFlow, ActionNodeName(node))
	}
	perf.inBody = true
	e.tokens = append(e.tokens, Token{ID: e.nextTokenID, Location: perf.graph.Initial, frame: perf})
	e.nextTokenID++
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeEnter(ActionNodeName(node))
	}
	for perf.live > 0 {
		if err := e.ctx.incrementStep(); err != nil {
			return err
		}
		moved, err := e.stepSubflow(perf)
		if err != nil {
			return err
		}
		if moved {
			continue
		}
		if len(e.waitingTokens(perf)) > 0 {
			return e.deadlockError(perf)
		}
		return fmt.Errorf("%w: %d token(s) stuck in %s, no progress made",
			ErrActionDeadlock, len(e.tokensIn(perf)), perf.describe())
	}
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeExit(ActionNodeName(node))
	}
	return nil
}

// stepSubflow steps every token of perf's flow once and reports whether any moved,
// which a retired, forked or relocated token did.
func (e *ActionExecutor) stepSubflow(perf *actionFrame) (bool, error) {
	before := e.subflowLocations(perf)
	for i := len(e.tokens) - 1; i >= 0; i-- {
		if i >= len(e.tokens) || !e.tokens[i].inFlowOf(perf) {
			continue
		}
		if err := e.stepToken(i); err != nil {
			return false, err
		}
	}
	after := e.subflowLocations(perf)
	if len(after) != len(before) {
		return true, nil
	}
	for id, location := range after {
		if before[id] != location {
			return true, nil
		}
	}
	return false, nil
}

// subflowLocations returns where each token of perf's flow sits, by token ID.
func (e *ActionExecutor) subflowLocations(perf *actionFrame) map[int64]ast.Node {
	locations := make(map[int64]ast.Node)
	for _, idx := range e.tokensIn(perf) {
		locations[e.tokens[idx].ID] = e.tokens[idx].Location
	}
	return locations
}

// tokensIn returns the indices of the tokens running in perf's flow or one nested
// in it; every token's for nil.
func (e *ActionExecutor) tokensIn(perf *actionFrame) []int {
	var indices []int
	for i := range e.tokens {
		if e.tokens[i].inFlowOf(perf) {
			indices = append(indices, i)
		}
	}
	return indices
}

// inFlowOf reports whether the token runs in perf's flow or one nested in it;
// every token does of nil.
func (t Token) inFlowOf(perf *actionFrame) bool {
	if perf == nil {
		return true
	}
	for f := t.frame; f != nil; f = f.parent {
		if f == perf {
			return true
		}
	}
	return false
}

// leaveSubflow returns a token to the node whose flow has just completed, ends
// that node's performance and takes the node's own succession.
func (e *ActionExecutor) leaveSubflow(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	frame := token.frame
	token.frame = frame.parent
	token.Location = frame.node
	token.Wait = nil
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeExit(ActionNodeName(frame.node))
	}
	if err := e.endPerformance(frame); err != nil {
		return err
	}
	return e.completeNode(tokenIdx, frame)
}

// validateSubflows reports a nested node whose own flow could not be built, in
// graph's flow or in a block flow a body of it states. It runs at initialize(),
// not at construction, per the error-timing contract.
func (e *ActionExecutor) validateSubflows(graph *lower.ActionGraph) error {
	for _, node := range graph.Nodes {
		if sub, owns := e.subflowOf(graph, node); owns {
			if sub.Err != nil {
				return fmt.Errorf("%w: action node %s: %w",
					ErrInvalidActionFlow, ActionNodeName(node), sub.Err)
			}
			if sub.Graph.Initial == nil {
				return fmt.Errorf("%w: no initial node found in action node %s",
					ErrInvalidActionFlow, ActionNodeName(node))
			}
			if err := e.validateSubflows(sub.Graph); err != nil {
				return err
			}
		}
		for _, block := range blockFlowsOf(graph.Bodies[node]) {
			if err := e.validateSubflows(block); err != nil {
				return err
			}
		}
	}
	return nil
}

// blockFlowsOf returns the flows the blocks among stmts state, the nested ones included.
func blockFlowsOf(stmts []lower.Statement) []*lower.ActionGraph {
	var flows []*lower.ActionGraph
	inBlock := func(block lower.Block) {
		if block.Graph != nil {
			flows = append(flows, block.Graph)
			return
		}
		flows = append(flows, blockFlowsOf(block.Statements)...)
	}
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case lower.If:
			inBlock(s.Then)
			if s.Else != nil {
				inBlock(*s.Else)
			}
		case lower.Loop:
			inBlock(s.Body)
		case lower.Block:
			inBlock(s)
		}
	}
	return flows
}

// subflowNodeNames returns the names of the nodes of every flow nested under
// graph, so a debugger can break on a step of a nested flow.
func (e *ActionExecutor) subflowNodeNames(graph *lower.ActionGraph) []string {
	var names []string
	for _, node := range graph.Nodes {
		sub, owns := e.subflowOf(graph, node)
		if !owns || sub.Graph == nil {
			continue
		}
		for _, inner := range sub.Graph.Nodes {
			names = append(names, ActionNodeNames(inner)...)
		}
		names = append(names, e.subflowNodeNames(sub.Graph)...)
	}
	return names
}

// connectionsOf returns the connectors a send in a performance may route through:
// the action's own, plus every enclosing flow's for a nested one.
func (e *ActionExecutor) connectionsOf(frame *actionFrame) []lower.Connection {
	return frame.connections
}

// joinConnections appends the connectors a nested flow declares to those around it.
func joinConnections(outer, inner []lower.Connection) []lower.Connection {
	if len(inner) == 0 {
		return outer
	}
	joined := make([]lower.Connection, 0, len(outer)+len(inner))
	joined = append(joined, outer...)
	return append(joined, inner...)
}
