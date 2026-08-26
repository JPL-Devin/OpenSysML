package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ActionNodes returns the nodes accepted by action lowering and whether the
// action has an initial node after interpreting `first <node>`.
func ActionNodes(actionDecl ast.Node, scope *symbols.Scope) (nodes []ast.Node, hasInitial bool, err error) {
	graph, _, _, err := collectActionNodes(actionDecl, scope)
	if err != nil {
		return nil, false, err
	}
	return graph.Nodes, graph.Initial != nil, nil
}

// ActionEndpointAccepted reports whether an action endpoint names a lowered
// node or one of the implicit start/done markers.
func ActionEndpointAccepted(nodes []ast.Node, hasInitial bool, ref ast.Node, source bool) bool {
	if findNodeByReference(nodes, ref) != nil {
		return true
	}
	return impliedMarker(ast.SimpleName(ref), source, !hasInitial)
}

func impliedMarker(name string, source, noInitial bool) bool {
	return (source && noInitial && name == "start") || (!source && name == "done")
}

func collectActionNodes(actionDecl ast.Node, scope *symbols.Scope) (*ActionGraph, []ast.Node, map[*ast.InitialNode]ast.Node, error) {
	graph := &ActionGraph{
		Scope:     scope,
		Nodes:     make([]ast.Node, 0),
		Edges:     make(map[ast.Node][]ActionEdge),
		DataFlows: make(map[ast.Node][]ObjectFlow),
		Bodies:    make(map[ast.Node][]Statement),
		Accepts:   make(map[ast.Node]Accept),
		Finals:    make([]ast.Node, 0),
	}

	var members []ast.Node
	switch n := actionDecl.(type) {
	case *ast.Usage:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	default:
		return nil, nil, nil, fmt.Errorf("action must be Usage or Definition, got %T", actionDecl)
	}

	// A succession can bind a member with no name of its own by position, which is
	// what puts a statement member (`then send …;`) in the token flow.
	sequenced := sequencedMembers(members)
	// First pass: collect nodes.
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			if graph.Initial != nil {
				return nil, nil, nil, fmt.Errorf("action has multiple initial nodes")
			}
			graph.Initial = n
			graph.Nodes = append(graph.Nodes, n)
			lowerNodeBody(graph, n, n.Members, scope)
		case *ast.FinalNode:
			graph.Finals = append(graph.Finals, n)
			graph.Nodes = append(graph.Nodes, n)
		case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode:
			graph.Nodes = append(graph.Nodes, n)
			lowerNodeBody(graph, n, ast.NodeBodyMembers(n), scope)
		case *ast.Usage:
			if n.Kind == ast.UsageAction {
				graph.Nodes = append(graph.Nodes, n)
				lowerBody(graph, n, childScope(scope, n))
			}
		case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode,
			*ast.SendStatement, *ast.TerminateStatement:
			// A statement written among the action's own members with no succession
			// binding it has no position in the token flow.
			if !sequenced[actualMember] {
				return nil, nil, nil, fmt.Errorf("%s written directly in an action body has no position in the token flow: declare it inside an action node", statementKeyword(n))
			}
			graph.Nodes = append(graph.Nodes, n)
			graph.Bodies[n] = []Statement{lowerStatement(n, scope)}
		}
	}

	graph.Connections = lowerConnections(members, OwnerBehavior, scope)
	graph.Attributes = lowerAttributes(members)
	// `first a then b;` names the node the flow starts at rather than declaring an
	// initial node, so a itself is the initial node and holds the edge.
	firstNode, err := resolveFirstNode(graph)
	if err != nil {
		return nil, nil, nil, err
	}
	return graph, members, firstNode, nil
}
