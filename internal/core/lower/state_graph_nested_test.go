package lower

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// stateUsageIn returns the first state usage declared in the package in src.
func stateUsageIn(t *testing.T, src string) *ast.Usage {
	t.Helper()
	_, machine := parseStateUsage(t, src)
	return machine
}

// parseStateUsage parses src and returns its root and the first state usage in
// it, which a caller needing a scope tree indexes the very same root for.
func parseStateUsage(t *testing.T, src string) (*ast.RootNamespace, *ast.Usage) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var found *ast.Usage
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, member := range members {
			if membership, ok := member.(*ast.Membership); ok {
				member = membership.Member
			}
			switch n := member.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Usage:
				if n.Kind == ast.UsageState && found == nil {
					found = n
				}
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatal("no state usage found")
	}
	return root, found
}

// A pseudostate declared inside a composite state is part of the graph and knows
// which state owns it, which is what a history pseudostate restores from.
func TestToStateGraph_NestedPseudostateOwner(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				state outer {
					state a;
					state b;
					choice pick;
				}
				start then outer;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	pick := graph.Pseudostates["pick"]
	if pick == nil {
		t.Fatal("pseudostate declared inside outer was not collected")
	}
	owner := graph.PseudostateOwner[pick]
	if owner == nil || owner.Name != "outer" {
		t.Fatalf("PseudostateOwner[pick] = %v, want outer", owner)
	}
}

// A pseudostate declared directly in the machine has no owning composite state.
func TestToStateGraph_TopLevelPseudostateHasNoOwner(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				choice pick;
				state a;
				start then pick;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	pick := graph.Pseudostates["pick"]
	if pick == nil {
		t.Fatal("choice pseudostate not collected")
	}
	if owner, ok := graph.PseudostateOwner[pick]; ok {
		t.Errorf("PseudostateOwner[pick] = %v, want no owner", owner)
	}
}

// An endpoint naming no vertex leaves its edge out of the graph rather than
// failing the lowering: the name-resolution tier reports the name, and a machine
// missing one edge still runs (UML 2.5.1 14.2.3.9 leniency).
func TestToStateGraph_EndpointNamingNoVertexLeavesTheEdgeOut(t *testing.T) {
	machine := stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				state busy;
				start then busy;
				transition busy to nowhere;
			}
		}
	`)
	graph, err := ToStateGraph(machine, nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	for from, transitions := range graph.Transitions {
		for _, transition := range transitions {
			if transition.Target == nil {
				t.Errorf("transition from %v was lowered with no target", from)
			}
		}
	}
	var busy *ast.StateNode
	for _, state := range graph.States {
		if state.Name == "busy" {
			busy = state
		}
	}
	if busy == nil {
		t.Fatal("busy was not collected")
	}
	if got := graph.Transitions[busy]; len(got) != 0 {
		t.Errorf("expected no transition out of busy, got %d", len(got))
	}
}

// An unqualified endpoint is resolved from where it was written: a transition
// inside beta naming work means beta's work, not the earlier alpha's.
func TestToStateGraph_UnqualifiedEndpointResolvesFromWhereItIsWritten(t *testing.T) {
	src := `
		package test {
			state Machine {
				initial start;
				state alpha {
					initial astart;
					state work;
					astart then work;
				}
				state beta {
					initial bstart;
					state work;
					bstart then work;
					transition work to done;
				}
				state done;
				start then beta;
			}
		}
	`
	root, machine := parseStateUsage(t, src)
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	scope := scopeOfNode(idx.DocumentRoot("test.sysml"), machine)
	if scope == nil {
		t.Fatal("the index has no scope for the machine")
	}
	graph, err := ToStateGraph(machine, scope)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	var from *ast.StateNode
	for node := range graph.Transitions {
		state, ok := node.(*ast.StateNode)
		if !ok || state.Name != "work" {
			continue
		}
		if graph.ParentState[state] != nil && graph.ParentState[state].Name == "beta" {
			from = state
		}
	}
	if from == nil {
		t.Fatal("expected the transition to leave beta's work, it leaves another state of that name")
	}
}

// scopeOfNode returns the scope node declares its members in.
func scopeOfNode(scope *symbols.Scope, node ast.Node) *symbols.Scope {
	if scope == nil {
		return nil
	}
	if scope.Node() == node {
		return scope
	}
	for _, child := range scope.Children() {
		if found := scopeOfNode(child, node); found != nil {
			return found
		}
	}
	return nil
}
