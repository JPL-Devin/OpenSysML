package lower

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
	switch member.(type) {
	case *ast.InitialNode, *ast.FinalNode, *ast.SuccessionEdge, *ast.ControlFlowEdge,
		*ast.ObjectFlowEdge, *ast.ForkNode, *ast.JoinNode, *ast.MergeNode,
		*ast.DecisionNode, *ast.ActionExecutionNode:
		return true
	default:
		return false
	}
}

// lowerBlockFlow lowers a block to the flow its members state: one node per
// action node, one node per run of plain statements between them, and one
// succession from each node to the node declared after it. scope is the block's
// own namespace.
func lowerBlockFlow(members []ast.Node, scope *symbols.Scope) *ActionGraph {
	graph := &ActionGraph{
		Scope:     scope,
		Nodes:     make([]ast.Node, 0, len(members)),
		Edges:     make(map[ast.Node][]ast.Node),
		Guards:    make(map[ast.Node]map[ast.Node]ast.Node),
		DataFlows: make(map[ast.Node][]ObjectFlow),
		Bodies:    make(map[ast.Node][]Statement),
		Accepts:   make(map[ast.Node]Accept),
		Finals:    make([]ast.Node, 0),

		StatementRuns: make(map[ast.Node]bool),
	}

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
		stmt, states := blockMemberStatement(actual, scope)
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

	graph.Connections = lowerConnections(members, OwnerBehavior)
	if len(graph.Nodes) > 0 {
		graph.Initial = graph.Nodes[0]
	}
	for i := 0; i+1 < len(graph.Nodes); i++ {
		graph.Edges[graph.Nodes[i]] = []ast.Node{graph.Nodes[i+1]}
	}
	return graph
}

// lowerFlowNode records what one action node of a block's flow runs: the action
// it performs, or the block a nested declaration states in a namespace of its own.
func lowerFlowNode(graph *ActionGraph, node ast.Node, scope *symbols.Scope) {
	switch n := node.(type) {
	case *ast.PerformActionNode:
		graph.Bodies[node] = []Statement{Effect{Kind: EffectPerform, Node: n, Scope: scope}}
	case *ast.Usage:
		if performsAction(n) {
			graph.Bodies[node] = []Statement{Effect{Kind: EffectPerform, Node: n, Scope: scope}}
			return
		}
		graph.Bodies[node] = []Statement{nestedActionBlock(n, childScope(scope, n))}
	default:
		graph.Bodies[node] = []Statement{lowerStatement(node, scope)}
	}
}

// nestedActionBlock lowers a nested action declared in a block: a block of its
// own, holding the statements its members state — or the flow they state in
// turn, which is how a nested action declares a nested action.
func nestedActionBlock(node *ast.Usage, scope *symbols.Scope) Block {
	if blockNeedsFlow(node.Members) {
		return Block{Node: node, Scope: scope, Graph: lowerBlockFlow(node.Members, scope)}
	}
	block := Block{Node: node, Scope: scope}
	for _, member := range node.Members {
		actual := unwrapMembership(member)
		if actual == nil || statesNoStep(actual) {
			continue
		}
		if stmt, states := blockMemberStatement(actual, scope); states {
			block.Statements = append(block.Statements, stmt)
		}
	}
	return block
}

// blockMemberStatement lowers a member of a block's flow that is a statement
// rather than an action node, and reports whether it states a step at all: a
// parameter carrying no value is bound by what performs the node declaring it.
func blockMemberStatement(member ast.Node, scope *symbols.Scope) (Statement, bool) {
	if usage, ok := member.(*ast.Usage); ok {
		if stmt, isParam := parameterStatement(usage, scope); isParam {
			return stmt, stmt != nil
		}
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

// parameterStatement lowers a parameter a node of a block's flow declares, and
// reports whether the usage was one. An input carrying a value declares it in
// the node's own frame, an output carrying one binds it outward, and a parameter
// carrying none is bound by what performs the node.
func parameterStatement(u *ast.Usage, scope *symbols.Scope) (Statement, bool) {
	if u.Direction == ast.DirNone && !u.IsResult {
		return nil, false
	}
	name, _ := ast.EffectiveName(u)
	if name == "" || u.Value == nil {
		return nil, true
	}
	if u.IsResult || u.Direction == ast.DirOut {
		return Assign{Target: name, Value: u.Value, Node: u, Scope: scope}, true
	}
	return Declare{Name: name, Value: u.Value, Node: u, Scope: scope}, true
}
