package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// An action node owning a flow runs that flow as its subperformances
// (`Actions::Action::subactions :> actions, subperformances`), which are
// suboccurrences of it and so end no later than it does. A token entering such a
// node moves into the node's performance and runs the node's own graph; the node
// completes, and its succession fires, only when the performance's last token
// retires (action_frame.go holds the performance itself).

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

// enterSubflow moves a token into the flow its node owns, run by the node's
// performance. The flow was validated at initialize(), so an unbuildable one
// here is reported rather than skipped.
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

// connectionsOf returns the connectors a send running in a performance may route
// through: the action's own for the root flow, and every enclosing flow's for a
// nested one, so a send deep in the nesting still reaches a connector declared
// around it.
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
