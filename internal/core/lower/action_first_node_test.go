package lower

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// `first a then b;` names the node the flow starts at, so a is the graph's
// initial node and holds the succession out of it.
func TestToActionGraph_FirstNamesADeclaredNode(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			action s2;
			first s1 then s2;
		}
	`)

	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")

	if graph.Initial != s1 {
		t.Errorf("initial node = %s, want s1", nodeDescription(graph.Initial))
	}
	if edges := graph.Edges[s1]; len(edges) != 1 || edges[0] != s2 {
		t.Errorf("s1 edges = %v, want [s2]", edges)
	}
	for _, node := range graph.Nodes {
		if _, ok := node.(*ast.InitialNode); ok {
			t.Error("the first end was kept as an initial node of its own")
		}
	}
}

// The same start written as its own member: the succession out of the first node
// is a separate member and still leaves from the named node.
func TestToActionGraph_FirstNamesADeclaredNodeSplit(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			action s2;
			first s1;
			then s1 s2;
		}
	`)

	s1 := nodeNamed(t, graph, "s1")
	s2 := nodeNamed(t, graph, "s2")

	if graph.Initial != s1 {
		t.Errorf("initial node = %s, want s1", nodeDescription(graph.Initial))
	}
	if edges := graph.Edges[s1]; len(edges) != 1 || edges[0] != s2 {
		t.Errorf("s1 edges = %v, want [s2]", edges)
	}
}

// A `first` end naming no declared node still declares an initial node of its own.
func TestToActionGraph_FirstDeclaresItsOwnInitialNode(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action s1;
			first start;
			then start s1;
		}
	`)

	initial, ok := graph.Initial.(*ast.InitialNode)
	if !ok {
		t.Fatalf("initial node = %T, want *ast.InitialNode", graph.Initial)
	}
	if initial.Name != "start" {
		t.Errorf("initial node name = %q, want %q", initial.Name, "start")
	}
	if edges := graph.Edges[initial]; len(edges) != 1 || edges[0] != nodeNamed(t, graph, "s1") {
		t.Errorf("initial edges = %v, want [s1]", edges)
	}
}

func nodeDescription(node ast.Node) string {
	if name := getNodeName(node); name != "" {
		return name
	}
	return "an unnamed node"
}
