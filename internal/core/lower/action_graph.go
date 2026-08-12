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

// Declare is a lowered declaration in a body-local block: `attribute i = 0;`
// written inside a loop or an `if` branch. The name it declares is a member of
// that block, so the executor binds it in the block's own frame and discards it
// when the block exits. Value is nil when the declaration carried none.
type Declare struct {
	Name  string
	Value ast.Node
	Node  ast.Node // the declaration itself, for diagnostics
}

func (Declare) statement() {}

// Block is a lowered body-local statement list: the body of a loop or of one
// branch of a conditional. It is a namespace of its own (symbols/builder.go), so
// the names its Declare statements introduce do not leak out of it.
type Block struct {
	Statements []Statement
	Node       ast.Node // the loop or branch the block belongs to
}

// Loop is a lowered loop statement. Kind says when the condition is tested:
// before each iteration (`while`), after each iteration (`loop … until`), or
// not at all, iteration being driven by a collection (`for`).
//
// Condition and Collection stay expressions because their values are only known
// at execution time. The iteration count is bounded by the executor's step
// budget, so a loop that never terminates fails the run rather than hanging it.
type Loop struct {
	Kind       ast.LoopKind
	Condition  ast.Node // nil for `for`, and for a `loop` written without `until`
	Variable   string   // `for` only: the name each element is bound to
	Collection ast.Node // `for` only: the collection iterated over
	Body       Block
	Node       ast.Node // the loop itself, for diagnostics
}

func (Loop) statement() {}

// If is a lowered conditional. The condition is evaluated in the enclosing
// body, outside both branches. Else is nil when the conditional declared none.
type If struct {
	Condition ast.Node
	Then      Block
	Else      *Block
	Node      ast.Node // the conditional itself, for diagnostics
}

func (If) statement() {}

// Unsupported is a body member the lowering layer recognizes but cannot yet
// turn into an executable statement. It is lowered rather than dropped so that
// reaching it fails the execution with a diagnostic instead of silently
// producing a wrong answer. Description names the construct.
type Unsupported struct {
	Description string
	Node        ast.Node
}

func (Unsupported) statement() {}

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
		case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode, *ast.SendStatement:
			// A statement is executed as part of an action node's body; written
			// directly among the action's own members it has no name a
			// succession could reach, hence no position in the token flow.
			return nil, fmt.Errorf("%s written directly in an action body has no position in the token flow: declare it inside an action node", statementKeyword(n))
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
		case *ast.SendStatement, *ast.AssignmentActionNode, *ast.WhileLoopActionNode, *ast.IfActionNode:
			graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(m))
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

// lowerStatement lowers one executable body statement. Every form it recognizes
// is lowered losslessly; a form it does not becomes Unsupported, so the executor
// reports it rather than skipping it.
func lowerStatement(member ast.Node) Statement {
	switch m := member.(type) {
	case *ast.SendStatement:
		return Send{
			Message: m.Message,
			Target:  ast.SimpleName(m.Target),
			IsVia:   m.IsVia,
		}
	case *ast.AssignmentActionNode:
		return Assign{
			Target: ast.SimpleName(m.Target),
			Value:  m.Value,
			Node:   m,
		}
	case *ast.WhileLoopActionNode:
		return Loop{
			Kind:       m.Kind,
			Condition:  m.Condition,
			Variable:   m.Variable.Name,
			Collection: m.Collection,
			Body:       lowerBlock(m, m.Body),
			Node:       m,
		}
	case *ast.IfActionNode:
		lowered := If{Condition: m.Condition, Node: m}
		if m.Then != nil {
			lowered.Then = lowerBlock(m.Then, m.Then.Body)
		}
		if m.Else != nil {
			block := lowerBlock(m.Else, m.Else.Body)
			lowered.Else = &block
		}
		return lowered
	case *ast.Usage:
		// An attribute declared in a body-local block is a member of that block:
		// it holds a value the block's statements read and write.
		if name, _ := ast.EffectiveName(m); m.Kind == ast.UsageAttribute && name != "" {
			return Declare{Name: name, Value: m.Value, Node: m}
		}
		return Unsupported{Description: usageDescription(m), Node: m}
	default:
		return Unsupported{Description: fmt.Sprintf("%T", member), Node: member}
	}
}

// lowerBlock lowers the body of a loop or of one branch of a conditional. owner
// is the node the block belongs to, which is the element that owns the block's
// body-local namespace.
func lowerBlock(owner ast.Node, members []ast.Node) Block {
	block := Block{Node: owner}
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil {
			continue
		}
		block.Statements = append(block.Statements, lowerStatement(actual))
	}
	return block
}

// usageDescription names a usage declared where a statement was expected, for
// the error the executor reports when it reaches it.
func usageDescription(u *ast.Usage) string {
	kind := u.Kind.String()
	if name := getNodeName(u); name != "" {
		return fmt.Sprintf("%s usage %q", kind, name)
	}
	return fmt.Sprintf("anonymous %s usage", kind)
}

// acceptPort returns the port an accept action routes through
// (`action r accept msg : T via p`), which the parser records as a reference
// relationship on the accept action, or "" when it named none.
func acceptPort(node *ast.Usage) string {
	for _, rel := range node.Relationships {
		if rel == nil || rel.Kind != ast.RelVia {
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
		// An unnamed usage is named after the feature it references or redefines
		// (`perform increment;` is a node named increment).
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return n.Ident.ShortName
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

// statementKeyword names a body statement for a diagnostic.
func statementKeyword(node ast.Node) string {
	switch n := node.(type) {
	case *ast.WhileLoopActionNode:
		return "a '" + n.Kind.String() + "' loop"
	case *ast.IfActionNode:
		return "an 'if' conditional"
	case *ast.AssignmentActionNode:
		return "an assignment"
	case *ast.SendStatement:
		return "a 'send'"
	default:
		return fmt.Sprintf("a %T statement", n)
	}
}
