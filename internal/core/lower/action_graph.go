// Package lower provides AST → execution IR lowering.
// Converts declarative members (TransitionMember, EntryMember) to
// operational graphs (nodes + edges) that executors consume.
package lower

import (
	"fmt"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// ActionGraph is the execution IR for actions.
// Nodes represent control flow points, edges represent flow paths.
type ActionGraph struct {
	// Nodes in the graph (InitialNode, FinalNode, ExecutionNode, etc.)
	Nodes []ast.Node

	// Edges: source node → list of target nodes
	Edges map[ast.Node][]ast.Node

	// Guards: source → target → guard expression
	Guards map[ast.Node]map[ast.Node]ast.Node

	// DataFlows: source node → list of object flows
	DataFlows map[ast.Node][]ObjectFlow

	// InitialNode (required)
	Initial ast.Node

	// FinalNodes (may be multiple)
	Finals []ast.Node
}

// ObjectFlow represents a data flow edge between pins.
type ObjectFlow struct {
	SourcePin string
	TargetPin string
	Target    ast.Node
}

// ToActionGraph converts an action AST (Usage or Definition) to an ActionGraph.
// Returns error if graph is malformed (e.g., no initial node, dangling edges).
func ToActionGraph(actionDecl ast.Node) (*ActionGraph, error) {
	graph := &ActionGraph{
		Nodes:     make([]ast.Node, 0),
		Edges:     make(map[ast.Node][]ast.Node),
		Guards:    make(map[ast.Node]map[ast.Node]ast.Node),
		DataFlows: make(map[ast.Node][]ObjectFlow),
		Finals:    make([]ast.Node, 0),
	}

	// Extract members from Usage or Definition
	var members []ast.Node
	switch n := actionDecl.(type) {
	case *ast.Usage:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	default:
		return nil, fmt.Errorf("action must be Usage or Definition, got %T", actionDecl)
	}

	// First pass: collect nodes
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			if graph.Initial != nil {
				return nil, fmt.Errorf("action has multiple initial nodes")
			}
			graph.Initial = n
			graph.Nodes = append(graph.Nodes, n)
		case *ast.FinalNode:
			graph.Finals = append(graph.Finals, n)
			graph.Nodes = append(graph.Nodes, n)
		case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode:
			graph.Nodes = append(graph.Nodes, n)
		case *ast.Usage:
			// Nested action usage (treat as execution node)
			if n.Kind == ast.UsageAction {
				graph.Nodes = append(graph.Nodes, n)
			}
		}
	}

	// Note: Initial node is optional at graph construction time.
	// The executor's initialize() will validate and return the error if missing.

	// Second pass: build edges
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			// Handle implicit successor from `first X then Y` syntax
			if n.Successor != nil {
				targetNode := findNodeByName(graph.Nodes, n.Successor)
				if targetNode == nil {
					return nil, fmt.Errorf("initial node %s successor references undefined target %v", n.Name, n.Successor)
				}
				graph.Edges[n] = append(graph.Edges[n], targetNode)
				if n.Guard != nil {
					if graph.Guards[n] == nil {
						graph.Guards[n] = make(map[ast.Node]ast.Node)
					}
					graph.Guards[n][targetNode] = n.Guard
				}
			}
		case *ast.SuccessionEdge:
			sourceNode := findNodeByName(graph.Nodes, n.Source)
			targetNode := findNodeByName(graph.Nodes, n.Target)

			if sourceNode == nil {
				return nil, fmt.Errorf("succession edge references undefined source node %v", n.Source)
			}
			if targetNode == nil {
				return nil, fmt.Errorf("succession edge references undefined target node %v", n.Target)
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], targetNode)
		case *ast.ControlFlowEdge:
			sourceNode := findNodeByName(graph.Nodes, n.Source)
			targetNode := findNodeByName(graph.Nodes, n.Target)

			if sourceNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined source %v", n.Source)
			}
			if targetNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined target %v", n.Target)
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], targetNode)

			// Store guard expression
			if n.Guard != nil {
				if graph.Guards[sourceNode] == nil {
					graph.Guards[sourceNode] = make(map[ast.Node]ast.Node)
				}
				graph.Guards[sourceNode][targetNode] = n.Guard
			}
		case *ast.ObjectFlowEdge:
			sourceNode, sourcePin := parsePinReference(graph.Nodes, n.Source)
			targetNode, targetPin := parsePinReference(graph.Nodes, n.Target)

			if sourceNode == nil {
				return nil, fmt.Errorf("object flow edge references undefined source %v", n.Source)
			}
			if targetNode == nil {
				return nil, fmt.Errorf("object flow edge references undefined target %v", n.Target)
			}

			graph.DataFlows[sourceNode] = append(graph.DataFlows[sourceNode], ObjectFlow{
				SourcePin: sourcePin,
				TargetPin: targetPin,
				Target:    targetNode,
			})
		}
	}

	return graph, nil
}

// unwrapMembership extracts the actual member from a Membership wrapper.
func unwrapMembership(node ast.Node) ast.Node {
	if membership, ok := node.(*ast.Membership); ok {
		return membership.Member
	}
	return node
}

// findNodeByName looks up a node by its qualified name.
func findNodeByName(nodes []ast.Node, qname *ast.QualifiedName) ast.Node {
	if qname == nil || len(qname.Parts) == 0 {
		return nil
	}

	targetName := qname.Parts[len(qname.Parts)-1].Text
	for _, node := range nodes {
		nodeName := getNodeName(node)
		if nodeName == targetName {
			return node
		}
	}
	return nil
}

// getNodeName extracts the name from a node.
func getNodeName(node ast.Node) string {
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
	case *ast.Usage:
		return n.Ident.Name
	}
	return ""
}

// parsePinReference extracts node and pin name from a qualified reference.
// Format: "nodeName.pinName" or just "nodeName" (pin = "")
func parsePinReference(nodes []ast.Node, qname *ast.QualifiedName) (ast.Node, string) {
	if qname == nil || len(qname.Parts) == 0 {
		return nil, ""
	}

	if len(qname.Parts) == 1 {
		// Just node name
		node := findNodeByName(nodes, qname)
		return node, ""
	}

	// Node.pin format
	nodeName := qname.Parts[0].Text
	pinName := qname.Parts[1].Text

	nodeQname := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: nodeName}}}
	node := findNodeByName(nodes, nodeQname)
	return node, pinName
}
