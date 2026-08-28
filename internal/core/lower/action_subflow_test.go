package lower

import "testing"

// An action node stating a flow of its own carries that flow as a subgraph, so
// the executor runs its steps rather than treating the node as a leaf.
func TestActionNodeCarriesItsOwnFlow(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			out attribute legs : Integer;
			first leg;
			action leg {
				action step { assign legs := 3; }
				first step;
			}
		}
	`)

	leg := nodeNamed(t, graph, "leg")
	sub, owns := graph.Subflows[leg]
	if !owns {
		t.Fatalf("leg carries no subflow: %#v", graph.Subflows)
	}
	if sub.Err != nil {
		t.Fatalf("leg subflow: %v", sub.Err)
	}
	if len(graph.Bodies[leg]) != 0 {
		t.Errorf("leg lowered %d body statements, want none", len(graph.Bodies[leg]))
	}
	step := nodeNamed(t, sub.Graph, "step")
	if sub.Graph.Initial != step {
		t.Errorf("subflow initial = %v, want step", sub.Graph.Initial)
	}
	if len(sub.Graph.Bodies[step]) != 1 {
		t.Errorf("step lowered %d statements, want 1", len(sub.Graph.Bodies[step]))
	}
}

// Nesting is not limited to one level: each node stating a flow carries it.
func TestActionNodeSubflowsNest(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			out attribute legs : Integer;
			first outerLeg;
			action outerLeg {
				first innerLeg;
				action innerLeg {
					action step { assign legs := 7; }
					first step;
				}
			}
		}
	`)

	outer := subflowOf(t, graph, "outerLeg")
	inner := subflowOf(t, outer, "innerLeg")
	if inner.Initial == nil {
		t.Error("innermost flow has no initial node")
	}
}

// A node stating no flow keeps its leaf lowering: its statements are its body.
func TestActionNodeWithoutFlowStaysALeaf(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			out attribute legs : Integer;
			first leg;
			action leg { assign legs := 1; }
		}
	`)

	leg := nodeNamed(t, graph, "leg")
	if _, owns := graph.Subflows[leg]; owns {
		t.Error("leaf node carries a subflow")
	}
	if len(graph.Bodies[leg]) != 1 {
		t.Errorf("leaf node lowered %d statements, want 1", len(graph.Bodies[leg]))
	}
}

// A node whose flow cannot be built carries the failure rather than losing it,
// so the executor reports it at initialize() instead of running a silent leaf.
func TestActionNodeSubflowCarriesItsFailure(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first leg;
			action leg {
				action step;
				succession first step then missing;
			}
		}
	`)

	leg := nodeNamed(t, graph, "leg")
	sub, owns := graph.Subflows[leg]
	if !owns {
		t.Fatal("leg carries no subflow")
	}
	if sub.Err == nil {
		t.Fatal("dangling inner succession lowered without an error")
	}
}

func subflowOf(t *testing.T, graph *ActionGraph, name string) *ActionGraph {
	t.Helper()
	node := nodeNamed(t, graph, name)
	sub, owns := graph.Subflows[node]
	if !owns {
		t.Fatalf("node %s carries no subflow", name)
	}
	if sub.Err != nil {
		t.Fatalf("node %s subflow: %v", name, sub.Err)
	}
	return sub.Graph
}
