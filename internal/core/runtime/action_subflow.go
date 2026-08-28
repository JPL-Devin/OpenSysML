package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// An action node owning a flow runs that flow as its subperformances
// (`Actions::Action::subactions :> actions, subperformances`), which are
// suboccurrences of it and so end no later than it does. A token entering such a
// node pushes a frame and runs the node's own graph; the node completes, and its
// succession fires, only when the frame's last token retires.

// actionFrame is one activation of a nested action node's own flow.
type actionFrame struct {
	node   ast.Node
	graph  *lower.ActionGraph
	parent *actionFrame
	// live counts the tokens still running in this activation, which a fork
	// inside it raises and a join or a retiring token lowers.
	live int
}

// graphOf returns the flow a frame runs, the action's own for the root frame.
func (e *ActionExecutor) graphOf(frame *actionFrame) *lower.ActionGraph {
	if frame == nil {
		return e.graph
	}
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

// enterSubflow moves a token into the flow its node owns. The flow was validated
// at initialize(), so an unbuildable one here is reported rather than skipped.
func (e *ActionExecutor) enterSubflow(tokenIdx int, sub *lower.Subflow) error {
	token := &e.tokens[tokenIdx]
	node := token.Location
	if sub.Graph == nil || sub.Graph.Initial == nil {
		return fmt.Errorf("%w: action node %s owns a flow that cannot be built",
			ErrInvalidActionFlow, ActionNodeName(node))
	}
	token.frame = &actionFrame{node: node, graph: sub.Graph, parent: token.frame, live: 1}
	token.Location = sub.Graph.Initial
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeEnter(ActionNodeName(node))
	}
	return nil
}

// leaveSubflow returns a token to the node whose flow has just completed and
// takes that node's own succession.
func (e *ActionExecutor) leaveSubflow(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	frame := token.frame
	token.frame = frame.parent
	token.Location = frame.node
	token.Wait = nil
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeExit(ActionNodeName(frame.node))
	}
	return e.completeNode(tokenIdx, frame.node)
}

// validateSubflows reports a nested node whose own flow could not be built. It
// runs at initialize(), not at construction, per the error-timing contract.
func (e *ActionExecutor) validateSubflows(graph *lower.ActionGraph) error {
	for _, node := range graph.Nodes {
		sub, owns := e.subflowOf(graph, node)
		if !owns {
			continue
		}
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
	return nil
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

// connectionsOf returns the connectors a send in the given flow may route
// through: the action's own, plus those the nested flow declares.
func (e *ActionExecutor) connectionsOf(graph *lower.ActionGraph) []lower.Connection {
	if graph == e.graph || len(graph.Connections) == 0 {
		return e.graph.Connections
	}
	joined := make([]lower.Connection, 0, len(e.graph.Connections)+len(graph.Connections))
	joined = append(joined, e.graph.Connections...)
	return append(joined, graph.Connections...)
}
