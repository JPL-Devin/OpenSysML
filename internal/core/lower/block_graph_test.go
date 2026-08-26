package lower

import (
	"testing"
)

// A block declaring an action node is lowered to a flow of its own: the runs of
// statements and the action nodes between them are its nodes, in declaration
// order, each succeeded by the next.
func TestBlockDeclaringAnActionNodeIsLoweredToAFlow(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				while total < 3 {
					assign total := total + 1;
					assign total := total * 2;
					action bump {
						assign total := total + 1;
					}
					perform other;
					assign total := total - 1;
				}
			}
			done end;
			then start accumulate;
			then accumulate end;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph == nil {
		t.Fatalf("loop body lowered to statements, want a flow of its own: %#v", block)
	}
	if len(block.Statements) != 0 {
		t.Errorf("loop body kept %d statements beside its flow", len(block.Statements))
	}

	flow := block.Graph
	if len(flow.Nodes) != 4 {
		t.Fatalf("block flow has %d nodes, want 4: %#v", len(flow.Nodes), flow.Nodes)
	}
	if flow.Initial != flow.Nodes[0] {
		t.Errorf("block flow starts at %T, want its first node %T", flow.Initial, flow.Nodes[0])
	}
	for i := 0; i+1 < len(flow.Nodes); i++ {
		successors := flow.Edges[flow.Nodes[i]]
		if len(successors) != 1 || successors[0].Target != flow.Nodes[i+1] {
			t.Errorf("node %d succeeds to %v, want the node written after it", i, successors)
		}
	}
	if last := flow.Edges[flow.Nodes[3]]; len(last) != 0 {
		t.Errorf("the last node succeeds to %v, want nothing", last)
	}

	// The two assignments written together are one step of the flow, so what the
	// first of them declares is in scope for the second.
	if !flow.StatementRuns[flow.Nodes[0]] {
		t.Errorf("the first node is not a run of statements: %#v", flow.Nodes[0])
	}
	if got := len(flow.Bodies[flow.Nodes[0]]); got != 2 {
		t.Errorf("the first node runs %d statements, want the 2 written together", got)
	}

	// The nested action declaration is a block of its own, holding its statements
	// in the namespace it declares.
	nested, ok := flow.Bodies[flow.Nodes[1]][0].(Block)
	if !ok {
		t.Fatalf("nested action lowered to %#v, want a block of its own", flow.Bodies[flow.Nodes[1]][0])
	}
	if len(nested.Statements) != 1 {
		t.Errorf("nested action holds %d statements, want 1", len(nested.Statements))
	}
	if flow.StatementRuns[flow.Nodes[1]] {
		t.Errorf("the nested action node is recorded as a run of statements")
	}

	// The `perform` is a node of the flow, performed when the token reaches it.
	effect, ok := flow.Bodies[flow.Nodes[2]][0].(Effect)
	if !ok || effect.Kind != EffectPerform {
		t.Errorf("perform lowered to %#v, want a perform effect", flow.Bodies[flow.Nodes[2]][0])
	}
}

// A nested action declared in a block declares a flow of its own in turn, so a
// block's flow nests as deeply as the model writes it.
func TestBlockFlowNestsTwoLevels(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				while total < 3 {
					action outer {
						assign total := total + 1;
						action inner {
							assign total := total * 2;
						}
					}
				}
			}
			done end;
			then start accumulate;
			then accumulate end;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph == nil {
		t.Fatalf("loop body lowered to statements, want a flow of its own: %#v", block)
	}
	outer, ok := block.Graph.Bodies[block.Graph.Nodes[0]][0].(Block)
	if !ok {
		t.Fatalf("outer action lowered to %#v, want a block", block.Graph.Bodies[block.Graph.Nodes[0]][0])
	}
	if outer.Graph == nil {
		t.Fatalf("outer action lowered to statements, want a flow of its own: %#v", outer)
	}
	if len(outer.Graph.Nodes) != 2 {
		t.Fatalf("outer flow has %d nodes, want the assignment and the nested action", len(outer.Graph.Nodes))
	}
	inner, ok := outer.Graph.Bodies[outer.Graph.Nodes[1]][0].(Block)
	if !ok {
		t.Fatalf("inner action lowered to %#v, want a block", outer.Graph.Bodies[outer.Graph.Nodes[1]][0])
	}
	if len(inner.Statements) != 1 {
		t.Errorf("inner action holds %d statements, want 1", len(inner.Statements))
	}
}

// A block stating an edge of its own is not a flow of its own: the flow it states
// is an action body's, so its members keep their statement form and the ones no
// statement executes are reported when reached.
func TestBlockStatingItsOwnEdgeKeepsItsStatements(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				while total < 3 {
					action bump {
						assign total := total + 1;
					}
					then bump bump;
				}
			}
			done end;
			then start accumulate;
			then accumulate end;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph != nil {
		t.Fatalf("loop body lowered to a flow of its own, want statements: %#v", block.Graph.Nodes)
	}
	if len(block.Statements) == 0 {
		t.Fatal("loop body lowered to no statements")
	}
	if _, ok := block.Statements[0].(Unsupported); !ok {
		t.Errorf("nested action lowered to %#v, want Unsupported", block.Statements[0])
	}
}

// A block's parameters are lowered where the block's flow reaches them: an input
// carrying a value declares it, an output carrying one binds it outward.
func TestNestedActionParametersAreLowered(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				for i in 1..3 {
					action bump {
						in stride = 2;
						out reached : Integer;
						assign total := total + stride;
						out result = total;
					}
				}
			}
			done end;
			then start accumulate;
			then accumulate end;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph == nil {
		t.Fatalf("loop body lowered to statements, want a flow of its own: %#v", block)
	}
	nested, ok := block.Graph.Bodies[block.Graph.Nodes[0]][0].(Block)
	if !ok {
		t.Fatalf("nested action lowered to %#v, want a block", block.Graph.Bodies[block.Graph.Nodes[0]][0])
	}
	if len(nested.Statements) != 3 {
		t.Fatalf("nested action holds %d statements, want 3: %#v", len(nested.Statements), nested.Statements)
	}
	declared, ok := nested.Statements[0].(Declare)
	if !ok || declared.Name != "stride" {
		t.Errorf("input parameter lowered to %#v, want a declaration of stride", nested.Statements[0])
	}
	bound, ok := nested.Statements[2].(Assign)
	if !ok || bound.Target != "result" {
		t.Errorf("output parameter lowered to %#v, want an assignment to result", nested.Statements[2])
	}
}

// loopBodyOf returns the block the named action node's loop runs.
func loopBodyOf(t *testing.T, graph *ActionGraph, node string) Block {
	t.Helper()
	body := graph.Bodies[nodeNamed(t, graph, node)]
	if len(body) != 1 {
		t.Fatalf("action node %s lowered to %d statements, want the loop", node, len(body))
	}
	loop, ok := body[0].(Loop)
	if !ok {
		t.Fatalf("action node %s lowered to %#v, want a loop", node, body[0])
	}
	return loop.Body
}
