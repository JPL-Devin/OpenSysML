package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A block — a loop body or a branch of a conditional — whose members include an
// action node (a nested action declaration, a `perform`) is lowered to a token
// flow of its own: an ActionGraph whose nodes are the block's steps in
// declaration order, each succeeded by the next, a maximal run of plain
// statements being one step.

// Steps returns the statements the block runs, wherever they live: its statement
// list, or the bodies of the nodes of its own token flow, in declaration order.
func (block Block) Steps() []Statement {
	if block.Graph == nil {
		return block.Statements
	}
	steps := make([]Statement, 0, len(block.Graph.Nodes))
	for _, node := range block.Graph.Nodes {
		steps = append(steps, block.Graph.Bodies[node]...)
	}
	return steps
}

// blockNeedsFlow reports whether a block's members make it a token flow of its
// own: some member is an action node, and none states a flow only an action body
// declares.
func blockNeedsFlow(members []ast.Node) bool {
	flow := false
	for _, member := range members {
		actual := unwrapMembership(member)
		if outsideBlockFlow(actual) {
			return false
		}
		if isFlowNode(actual) {
			flow = true
		}
	}
	return flow
}

// isFlowNode reports whether a block member is a node of the block's flow rather
// than a statement in it: a `perform`, or a nested action declaration. The
// block's own body parameter and an accept parameter are neither.
func isFlowNode(member ast.Node) bool {
	switch m := member.(type) {
	case *ast.PerformActionNode:
		return true
	case *ast.Usage:
		return m.Kind == ast.UsageAction && !m.IsBodyParameter && !m.IsAccept
	default:
		return false
	}
}

// outsideBlockFlow reports whether a member states a flow only an action body
// declares — an edge, a fork, a join, a decision, a start or an end. A block
// declaring one keeps its statement form, so it is reported rather than
// half-executed.
func outsideBlockFlow(member ast.Node) bool {
	switch m := member.(type) {
	case *ast.InitialNode, *ast.FinalNode, *ast.SuccessionEdge, *ast.ControlFlowEdge,
		*ast.ObjectFlowEdge, *ast.TransitionMember, *ast.ForkNode, *ast.JoinNode, *ast.MergeNode,
		*ast.DecisionNode, *ast.ActionExecutionNode:
		return true
	case *ast.Usage:
		return m.Kind == ast.UsageSuccession
	default:
		return false
	}
}

// lowerBlockFlow lowers a block to the flow its members state: a node per action
// node, a node per run of plain statements, a succession in declaration order.
// nodeBody says the members are a nested action's own, so a parameter is its own.
func lowerBlockFlow(members []ast.Node, scope *symbols.Scope, nodeBody bool) *ActionGraph {
	graph := &ActionGraph{
		Scope:     scope,
		Nodes:     make([]ast.Node, 0, len(members)),
		Edges:     make(map[ast.Node][]ActionEdge),
		DataFlows: make(map[ast.Node][]ObjectFlow),
		Bodies:    make(map[ast.Node][]Statement),
		Accepts:   make(map[ast.Node]Accept),
		Finals:    make([]ast.Node, 0),

		StatementRuns: make(map[ast.Node]bool),
	}

	// The action nodes are known up front so that a connector written before the
	// node it addresses (`bind add.a = x;` above `action add`) still finds it.
	nodes := flowNodesAmong(members)

	// run is the node the statements written between two action nodes belong to:
	// they are one step of the flow, sharing the frame the block gives it.
	var run ast.Node
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil || statesNoStep(actual) {
			continue
		}
		if isFlowNode(actual) {
			run = nil
			graph.Nodes = append(graph.Nodes, actual)
			lowerFlowNode(graph, actual, scope)
			continue
		}
		stmt, states := blockStep(graph, nodes, actual, scope, nodeBody)
		if !states {
			continue
		}
		if run == nil {
			run = actual
			graph.Nodes = append(graph.Nodes, actual)
			graph.StatementRuns[actual] = true
		}
		graph.Bodies[run] = append(graph.Bodies[run], stmt)
	}

	if len(graph.Nodes) > 0 {
		graph.Initial = graph.Nodes[0]
	}
	for i := 0; i+1 < len(graph.Nodes); i++ {
		graph.Edges[graph.Nodes[i]] = []ActionEdge{{Target: graph.Nodes[i+1]}}
	}
	recordBlockNodes(graph)
	return graph
}

// recordBlockNodes fills graph.BlockNodes from the bodies of its nodes.
func recordBlockNodes(graph *ActionGraph) {
	for node, body := range graph.Bodies {
		nodes := blockNodesOf(body, nil)
		if len(nodes) == 0 {
			continue
		}
		if graph.BlockNodes == nil {
			graph.BlockNodes = make(map[ast.Node][]ast.Node)
		}
		graph.BlockNodes[node] = nodes
	}
}

// blockNodesOf appends the action nodes the blocks among stmts declare, in
// declaration order; a node's own body declares its subperformances, not these.
func blockNodesOf(stmts []Statement, into []ast.Node) []ast.Node {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case If:
			into = blockNodesIn(s.Then, into)
			if s.Else != nil {
				into = blockNodesIn(*s.Else, into)
			}
		case Loop:
			into = blockNodesIn(s.Body, into)
		case Block:
			into = blockNodesIn(s, into)
		}
	}
	return into
}

// blockNodesIn appends the action nodes one block declares: the action usages
// among the nodes of its flow, and those the blocks in its statement runs declare.
func blockNodesIn(block Block, into []ast.Node) []ast.Node {
	if block.Graph == nil {
		return blockNodesOf(block.Statements, into)
	}
	for _, node := range block.Graph.Nodes {
		if _, isUsage := node.(*ast.Usage); isUsage && !block.Graph.StatementRuns[node] {
			into = append(into, node)
			continue
		}
		into = blockNodesOf(block.Graph.Bodies[node], into)
	}
	return into
}

// lowerFlowNode records what one action node of a block's flow runs: the action
// a `perform` names, or the features and body of a nested action declaration.
func lowerFlowNode(graph *ActionGraph, node ast.Node, scope *symbols.Scope) {
	switch n := node.(type) {
	case *ast.PerformActionNode:
		graph.Bodies[node] = []Statement{Effect{Kind: EffectPerform, Node: n, Scope: scope}}
	case *ast.Usage:
		// An accept node suspends the action it is a node of, which a block's flow
		// has no token to park; it is lowered as unsupported so that reaching it is
		// reported rather than passed over.
		if acceptsMessage(n) {
			graph.Bodies[node] = []Statement{Unsupported{
				Description: "'accept' in a loop or branch body",
				Node:        n,
				Scope:       scope,
			}}
			return
		}
		lowerNestedNode(graph, n, childScope(scope, n))
	default:
		graph.Bodies[node] = []Statement{lowerStatement(node, scope)}
	}
}

// acceptsMessage reports whether an action node declares an accept payload, so
// that reaching it waits for a message (SysML.xtext AcceptNode).
func acceptsMessage(node *ast.Usage) bool {
	for _, member := range node.Members {
		if u, ok := unwrapMembership(member).(*ast.Usage); ok && u.IsAccept {
			return true
		}
	}
	return false
}

// lowerNestedNode records a nested action declared in a block like a node of the
// action's flow: the features it declares, and the statements or flow its members state.
func lowerNestedNode(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	lowerFeatures(graph, node, scope)
	if blockNeedsFlow(node.Members) {
		graph.Bodies[node] = []Statement{Block{
			Node:  node,
			Scope: scope,
			Graph: lowerBlockFlow(node.Members, scope, true),
		}}
		return
	}
	for _, member := range node.Members {
		actual := unwrapMembership(member)
		if actual == nil || statesNoStep(actual) {
			continue
		}
		if stmt, states := blockMemberStatement(actual, scope, true); states {
			graph.Bodies[node] = append(graph.Bodies[node], stmt)
		}
	}
}

// flowNodesAmong returns the members of a block that are nodes of its flow, in
// declaration order.
func flowNodesAmong(members []ast.Node) []ast.Node {
	var nodes []ast.Node
	for _, member := range members {
		if actual := unwrapMembership(member); actual != nil && isFlowNode(actual) {
			nodes = append(nodes, actual)
		}
	}
	return nodes
}

// blockStep lowers a member of a block's flow that is not a node of it, and reports
// whether it states a step: a connector at a node's pin is part of the flow, not a step.
func blockStep(graph *ActionGraph, nodes []ast.Node, member ast.Node, scope *symbols.Scope, nodeBody bool) (Statement, bool) {
	if usage, ok := member.(*ast.Usage); ok {
		if stmt, connects := lowerBlockConnector(graph, nodes, usage, scope); connects {
			return stmt, stmt != nil
		}
	}
	return blockMemberStatement(member, scope, nodeBody)
}

// lowerBlockConnector lowers a binding or flow at a pin of one of the block's nodes into
// graph and reports whether u is one; one that cannot be lowered becomes an Unsupported step.
func lowerBlockConnector(graph *ActionGraph, nodes []ast.Node, u *ast.Usage, scope *symbols.Scope) (Statement, bool) {
	switch {
	case u.Kind == ast.UsageBinding:
		bindings, err := lowerPinBindings(nodes, u, scope)
		if err != nil {
			return unsupportedConnector(u, scope, err), true
		}
		if len(bindings) == 0 {
			return nil, false
		}
		graph.Bindings = append(graph.Bindings, bindings...)
		return nil, true
	case u.Kind == ast.UsageFlow && u.FlowEnds != nil:
		from, _ := flowEnd(nodes, u.FlowEnds.From)
		to, _ := flowEnd(nodes, u.FlowEnds.To)
		if from == nil && to == nil {
			return nil, false
		}
		source, flow, err := lowerFlow(nodes, u)
		if err != nil {
			return unsupportedConnector(u, scope, err), true
		}
		graph.DataFlows[source] = append(graph.DataFlows[source], flow)
		return nil, true
	}
	return nil, false
}

// unsupportedConnector is the step a connector that could not be lowered becomes,
// so that reaching it reports why.
func unsupportedConnector(u *ast.Usage, scope *symbols.Scope, err error) Statement {
	return Unsupported{
		Description: fmt.Sprintf("%s (%v)", usageDescription(u), err),
		Node:        u,
		Scope:       scope,
	}
}

// blockMemberStatement lowers a member of a block's flow that is a statement, and
// reports whether it states a step: a nested action's own feature is not one.
func blockMemberStatement(member ast.Node, scope *symbols.Scope, nodeBody bool) (Statement, bool) {
	if usage, ok := member.(*ast.Usage); ok && nodeBody && DeclaresNodeFeature(usage) {
		return nil, false
	}
	return lowerStatement(member, scope), true
}

// statesNoStep reports whether a member declares something about the block
// rather than a step of it, so that reaching the block does not reach it.
func statesNoStep(member ast.Node) bool {
	switch member.(type) {
	case *ast.Documentation, *ast.Comment, *ast.Definition, *ast.Import:
		return true
	default:
		return false
	}
}
