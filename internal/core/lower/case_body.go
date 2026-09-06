package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A case body whose members include action nodes — a nested action, a `perform`
// — states a flow of steps (SysML v2 §7.21: a case is a calculation, so an
// action). The body runs as one sequential flow in the case's own frame: the
// nodes and the statement runs between them in declaration order, re-sequenced
// by the successions the body states (`then`), which are constraints on the
// order rather than a token flow of their own. The results run after the flow.

// PerformsSteps reports whether decl is a behavior whose body performs the action
// nodes among its members as steps: an analysis case (SysML v2 §7.22), which is
// a calculation and an action. A calc's body reads them as declarations only.
func PerformsSteps(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefAnalysisCase
	case *ast.Usage:
		return d.Kind == ast.UsageAnalysisCase
	default:
		return false
	}
}

// caseSteps lowers a body whose steps are action nodes to one Block over the
// body's own flow, followed by the results the body declares.
func caseSteps(owner ast.Node, body []ast.Node, scope *symbols.Scope) []Statement {
	var results []Statement
	graph := lowerBlockFlowWith(body, scope, func(graph *ActionGraph, nodes []ast.Node, member ast.Node) (Statement, bool) {
		if usage, ok := member.(*ast.Usage); ok {
			if stmt, connects := lowerBlockConnector(graph, nodes, usage, scope); connects {
				return stmt, stmt != nil
			}
		}
		stmt, states := caseMemberStep(member, scope)
		if !states {
			return nil, false
		}
		if _, isResult := stmt.(Return); isResult {
			results = append(results, stmt)
			return nil, false
		}
		return stmt, true
	})
	if err := sequenceCaseFlow(graph, body); err != nil {
		unsupported := Unsupported{
			Description: fmt.Sprintf("the successions of the body (%v)", err),
			Node:        owner,
			Scope:       scope,
		}
		return append([]Statement{unsupported}, results...)
	}
	flow := Block{Node: owner, Scope: scope, Graph: graph, Own: true}
	return append([]Statement{flow}, results...)
}

// caseMemberStep lowers a member of a case body that is not an action node: a
// start, an end or a succession only sequences the nodes, and a control node —
// which only a token flow executes — is a step reporting that it is not one.
func caseMemberStep(member ast.Node, scope *symbols.Scope) (Statement, bool) {
	switch m := member.(type) {
	case *ast.InitialNode, *ast.FinalNode, *ast.SuccessionEdge:
		return nil, false
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode,
		*ast.ControlFlowEdge, *ast.ActionExecutionNode, *ast.TransitionMember:
		return Unsupported{
			Description: controlNodeDescription(m) + " in a case body, which runs its steps in sequence",
			Node:        m,
			Scope:       scope,
		}, true
	case *ast.Usage:
		if m.Kind == ast.UsageSuccession {
			return Unsupported{
				Description: "a 'succession' usage in a case body, which sequences its steps by 'then'",
				Node:        m,
				Scope:       scope,
			}, true
		}
	}
	return calcStep(member, scope)
}

// controlNodeDescription names a control node or edge for a diagnostic.
func controlNodeDescription(node ast.Node) string {
	switch node.(type) {
	case *ast.ForkNode:
		return "a 'fork'"
	case *ast.JoinNode:
		return "a 'join'"
	case *ast.MergeNode:
		return "a 'merge'"
	case *ast.DecisionNode:
		return "a 'decide'"
	case *ast.ControlFlowEdge:
		return "a guarded succession"
	case *ast.TransitionMember:
		return "a transition"
	default:
		return "an action-execution node"
	}
}

// sequenceCaseFlow orders the nodes of a case body's declaration-order flow by
// the successions the body states: a node runs after every node a succession
// places before it, ties broken by declaration order, and a statement run stays
// after the action node written before it. Successions forming a cycle, or
// naming no node of the body, are an error.
func sequenceCaseFlow(graph *ActionGraph, body []ast.Node) error {
	edges, err := caseSuccessions(graph, body)
	if err != nil {
		return err
	}
	if len(edges) == 0 {
		return nil
	}

	// The action nodes are sequenced; each carries the statement runs written
	// after it, up to the next action node.
	var steps []ast.Node
	runs := make(map[ast.Node][]ast.Node)
	var leading []ast.Node
	var last ast.Node
	for _, node := range graph.Nodes {
		if graph.StatementRuns[node] {
			if last == nil {
				leading = append(leading, node)
			} else {
				runs[last] = append(runs[last], node)
			}
			continue
		}
		steps = append(steps, node)
		last = node
	}

	before := make(map[ast.Node]int, len(steps))
	after := make(map[ast.Node][]ast.Node, len(steps))
	for _, edge := range edges {
		before[edge.target]++
		after[edge.source] = append(after[edge.source], edge.target)
	}
	ordered := make([]ast.Node, 0, len(graph.Nodes))
	ordered = append(ordered, leading...)
	placed := make(map[ast.Node]bool, len(steps))
	for len(ordered) < len(graph.Nodes) {
		var next ast.Node
		for _, step := range steps {
			if !placed[step] && before[step] == 0 {
				next = step
				break
			}
		}
		if next == nil {
			return fmt.Errorf("the successions form a cycle among the steps")
		}
		placed[next] = true
		for _, target := range after[next] {
			before[target]--
		}
		ordered = append(ordered, next)
		ordered = append(ordered, runs[next]...)
	}

	explicit := make(map[ast.Node]map[ast.Node]ast.Node, len(edges))
	for _, edge := range edges {
		if explicit[edge.source] == nil {
			explicit[edge.source] = make(map[ast.Node]ast.Node)
		}
		explicit[edge.source][edge.target] = edge.decl
	}
	graph.Nodes = ordered
	graph.Initial = ordered[0]
	graph.Edges = make(map[ast.Node][]ActionEdge, len(ordered))
	for i := 0; i+1 < len(ordered); i++ {
		edge := ActionEdge{Target: ordered[i+1]}
		if decl, ok := explicit[ordered[i]][ordered[i+1]]; ok {
			edge.Decl = decl
		}
		graph.Edges[ordered[i]] = []ActionEdge{edge}
	}
	return nil
}

// caseSuccession is one succession a case body states between two of its nodes.
type caseSuccession struct {
	source, target ast.Node
	decl           ast.Node
}

// caseSuccessions reads the successions a case body states: `a then b`, a
// member-attached `then`, and `first a` or `start then a` placing a first.
func caseSuccessions(graph *ActionGraph, body []ast.Node) ([]caseSuccession, error) {
	var edges []caseSuccession
	var first ast.Node
	for _, member := range body {
		switch n := member.(type) {
		case *ast.InitialNode:
			if n.Successor == nil {
				continue
			}
			target := findNodeByReference(graph.Nodes, n.Successor)
			if target == nil {
				return nil, fmt.Errorf("'first' names %s, which is no step of the body", edgeEndName(n.Successor))
			}
			// `first a then b;` names the step the flow starts at rather than an
			// initial node of its own; `first a;` and `start then a;` place a first.
			if n.Name != "" {
				if source := stepNamed(graph.Nodes, n.Name); source != nil {
					if source == target {
						return nil, fmt.Errorf("a succession places %s after itself", n.Name)
					}
					edges = append(edges, caseSuccession{source: source, target: target, decl: n})
					continue
				}
			}
			first = target
		case *ast.SuccessionEdge:
			source, err := caseSuccessionEnd(graph, body, n.Source, n.SourceMember, true)
			if err != nil {
				return nil, err
			}
			target, err := caseSuccessionEnd(graph, body, n.Target, n.TargetMember, false)
			if err != nil {
				return nil, err
			}
			if source.marker && target.node != nil {
				first = target.node
			}
			if source.node == nil || target.node == nil {
				continue
			}
			if source.node == target.node {
				return nil, fmt.Errorf("a succession places %s after itself", edgeEnd(n.Source, n.SourceMember))
			}
			edges = append(edges, caseSuccession{source: source.node, target: target.node, decl: n})
		}
	}
	if first != nil {
		for _, node := range graph.Nodes {
			if node != first && !graph.StatementRuns[node] {
				edges = append(edges, caseSuccession{source: first, target: node})
			}
		}
	}
	return edges, nil
}

// stepNamed finds the step of the body answering to name, or nil.
func stepNamed(nodes []ast.Node, name string) ast.Node {
	for _, node := range nodes {
		if nodeAnswersTo(node, name) {
			return node
		}
	}
	return nil
}

// caseEnd is one end of a succession in a case body: the step it places, or none
// for a statement — placed by declaration order — or the implied start or done marker.
type caseEnd struct {
	node   ast.Node
	marker bool
}

// caseSuccessionEnd resolves one end of a succession among the body's steps; an
// end naming nothing the body declares is an error.
func caseSuccessionEnd(graph *ActionGraph, body []ast.Node, ref *ast.QualifiedName, member ast.Node, source bool) (caseEnd, error) {
	if member != nil {
		return caseEnd{node: resolveEnd(graph.Nodes, nil, member)}, nil
	}
	if ref == nil {
		return caseEnd{}, fmt.Errorf("a succession names no step of the body at one end")
	}
	if node := findNodeByReference(graph.Nodes, ref); node != nil {
		return caseEnd{node: node}, nil
	}
	name := ast.SimpleName(ref)
	if (source && name == "start") || (!source && name == "done") {
		return caseEnd{marker: true}, nil
	}
	for _, member := range body {
		if nodeAnswersTo(member, name) {
			return caseEnd{}, nil
		}
	}
	return caseEnd{}, fmt.Errorf("a succession names %s, which the body does not declare", edgeEndName(ref))
}
