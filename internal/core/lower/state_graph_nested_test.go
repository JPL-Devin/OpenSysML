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

	pick := pseudostateNamed(graph, "pick")
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

	pick := pseudostateNamed(graph, "pick")
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

// A succession names its endpoints the way a transition does, so one whose
// target is a pseudostate reaches it rather than being left out of the graph.
func TestSuccessionReachesAPseudostate(t *testing.T) {
	root, machine := parseStateUsage(t, `
		package test {
			state Machine {
				initial start;
				state busy;
				junction route;
				state done;

				start then busy;
				busy then route;
				route then done;
			}
		}
	`)
	graph := graphOf(t, root, machine)

	route := pseudostateNamed(graph, "route")
	if route == nil {
		t.Fatal("junction route was not collected")
	}
	if len(graph.Transitions[route]) != 1 {
		t.Fatalf("transitions out of route = %d, want the succession `route then done`", len(graph.Transitions[route]))
	}
	if target := graph.Transitions[route][0].Target; target != ast.Node(stateNamed(graph, "done")) {
		t.Fatalf("route leads to %v, want done", target)
	}
	if !leadsTo(graph, stateNamed(graph, "busy"), route) {
		t.Fatal("the succession `busy then route` is not in the graph")
	}
}

// A succession qualifying its target reaches the vertex it names, not the first
// one of that simple name: two composite states may each declare a `work`.
func TestSuccessionQualifiedTargetNamesTheVertexItQualifies(t *testing.T) {
	root, machine := parseStateUsage(t, `
		package test {
			state Machine {
				initial start;
				state alpha {
					state work;
				}
				state beta {
					state work;
				}

				start then alpha;
				alpha then beta::work;
			}
		}
	`)
	graph := graphOf(t, root, machine)

	alpha := stateNamed(graph, "alpha")
	if len(graph.Transitions[alpha]) != 1 {
		t.Fatalf("transitions out of alpha = %d, want the succession `alpha then beta::work`", len(graph.Transitions[alpha]))
	}
	target, ok := graph.Transitions[alpha][0].Target.(*ast.StateNode)
	if !ok {
		t.Fatalf("alpha leads to %T, want a state", graph.Transitions[alpha][0].Target)
	}
	if owner := graph.ParentState[target]; owner == nil || owner.Name != "beta" {
		t.Fatalf("alpha leads to the work of %v, want beta's", owner)
	}
}

// Two regions may declare same-named pseudostates, and each is its own vertex.
func TestSameNamedPseudostatesInSiblingRegionsAreBothCollected(t *testing.T) {
	root, machine := parseStateUsage(t, `
		package test {
			state Machine {
				initial start;
				state running {
					region left {
						initial lstart;
						state lidle;
						junction pick;
						lstart then lidle;
						lidle then pick;
						pick then lidle;
					}
					region right {
						initial rstart;
						state ridle;
						junction pick;
						rstart then ridle;
						ridle then pick;
						pick then ridle;
					}
				}
				start then running;
			}
		}
	`)
	graph := graphOf(t, root, machine)

	var picks []*ast.PseudostateNode
	for _, ps := range graph.Pseudostates {
		if ps.Name == "pick" {
			picks = append(picks, ps)
		}
	}
	if len(picks) != 2 {
		t.Fatalf("pseudostates named pick = %d, want one per region", len(picks))
	}
	if picks[0] == picks[1] {
		t.Fatal("the two regions share one pseudostate node")
	}
	for _, pick := range picks {
		if len(graph.Transitions[pick]) != 1 {
			t.Fatalf("transitions out of a pick = %d, want its own region's", len(graph.Transitions[pick]))
		}
		target, ok := graph.Transitions[pick][0].Target.(*ast.StateNode)
		if !ok {
			t.Fatalf("a pick leads to %T, want a state", graph.Transitions[pick][0].Target)
		}
		if graph.RegionOf[target] != graph.RegionOf[graph.PseudostateOwner[pick]] &&
			graph.RegionOf[target] == nil {
			t.Fatalf("a pick leads to %s, which no region declares", target.Name)
		}
	}
}

// graphOf lowers the machine src declares with the scope tree of its document.
func graphOf(t *testing.T, root *ast.RootNamespace, machine *ast.Usage) *StateGraph {
	t.Helper()
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	scope := scopeOfNode(idx.DocumentRoot("test.sysml"), machine)
	if scope == nil {
		t.Fatal("the index has no scope for the machine")
	}
	graph, err := ToStateGraph(machine, scope)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	return graph
}

// stateNamed is the graph's state with that simple name, or nil.
func stateNamed(graph *StateGraph, name string) *ast.StateNode {
	for _, state := range graph.States {
		if state.Name == name {
			return state
		}
	}
	return nil
}

// leadsTo reports whether a transition of the graph goes from source to target.
func leadsTo(graph *StateGraph, source, target ast.Node) bool {
	for _, trans := range graph.Transitions[source] {
		if trans.Target == target {
			return true
		}
	}
	return false
}
