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

	// Bodies: node → the statements that node executes, in declaration order
	Bodies map[ast.Node][]Statement

	// Accepts: node → the message that node waits for
	Accepts map[ast.Node]Accept

	// InitialNode (required)
	Initial ast.Node

	// FinalNodes (may be multiple)
	Finals []ast.Node

	// Connections are the connectors declared in the action body, which is how
	// a `send ... via <port>` finds the ports it reaches.
	Connections []Connection
}

// Statement is one lowered statement in an action node's body. Statements are
// kept in declaration order so the executor never walks the node's members
// again to find them.
type Statement interface {
	statement()
}

// Send is a lowered send statement. Message stays an expression because its
// value is only known at execution time.
//
// Target is the simple name the send addressed, empty for a broadcast. IsVia
// records that the name is a port of the sender rather than a receiver, in which
// case the message goes to whatever the graph's Connections join that port to.
type Send struct {
	Message ast.Node
	Target  string
	IsVia   bool
}

func (Send) statement() {}

// Assign is a lowered assignment: `assign <Target> := <Value>`. Target is the
// simple name assigned to, empty when the target was not a plain name.
type Assign struct {
	Target string
	Value  ast.Node
	Node   ast.Node // the statement itself, for diagnostics
}

func (Assign) statement() {}

// Accept is a lowered accept parameter: `action r accept msg : Warning;`.
// SignalType is the parameter's declared type, empty when it was declared
// without one, in which case the node accepts a message of any type.
//
// ViaPort is the port named by `accept msg : Warning via p`, empty when the
// accept named none. A port-routed message is only offered to an accept on the
// port it arrived at, so the two forms do not consume each other's messages.
type Accept struct {
	ParamName  string
	SignalType string
	ViaPort    string
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
		Bodies:    make(map[ast.Node][]Statement),
		Accepts:   make(map[ast.Node]Accept),
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
				lowerBody(graph, n)
			}
		}
	}

	graph.Connections = lowerConnections(members)

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

// lowerBody records a nested action node's statements and the message it waits
// for, so the executor reads them from the graph rather than walking the node's
// members again.
func lowerBody(graph *ActionGraph, node *ast.Usage) {
	for _, member := range node.Members {
		switch m := unwrapMembership(member).(type) {
		case *ast.SendStatement:
			graph.Bodies[node] = append(graph.Bodies[node], Send{
				Message: m.Message,
				Target:  ast.SimpleName(m.Target),
				IsVia:   m.IsVia,
			})
		case *ast.AssignmentActionNode:
			graph.Bodies[node] = append(graph.Bodies[node], Assign{
				Target: ast.SimpleName(m.Target),
				Value:  m.Value,
				Node:   m,
			})
		case *ast.Usage:
			if !m.IsAccept {
				continue
			}
			graph.Accepts[node] = Accept{
				ParamName:  m.Ident.Name,
				SignalType: typingTarget(m),
				ViaPort:    acceptPort(node),
			}
		}
	}
}

// acceptPort returns the port an accept action routes through
// (`action r accept msg : T via p`), which the parser records as a reference
// relationship on the accept action, or "" when it named none.
func acceptPort(node *ast.Usage) string {
	for _, rel := range node.Relationships {
		if rel == nil || rel.Kind != ast.RelReferences {
			continue
		}
		if name := ast.SimpleName(rel.Target); name != "" {
			return name
		}
	}
	return ""
}

// typingTarget returns the name a usage was typed with (`: T`), or "" when it
// was declared without a type.
func typingTarget(usage *ast.Usage) string {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		if name := ast.SimpleName(rel.Target); name != "" {
			return name
		}
	}
	return ""
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
