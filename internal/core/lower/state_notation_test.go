package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// The textual `defer` notation reaches the graph as the deferral of the state
// declaring it, normalized the same way a transition trigger is.
func TestToStateGraph_DeferNotation(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				state busy {
					defer Ping, setSpeed(value);
				}
				start then busy;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	var busy *ast.StateNode
	for _, state := range graph.States {
		if state.Name == "busy" {
			busy = state
		}
	}
	if busy == nil {
		t.Fatal("state busy is not in the graph")
	}

	deferred := graph.Deferred[busy]
	if len(deferred) != 2 {
		t.Fatalf("expected busy to defer 2 events, got %d", len(deferred))
	}
	accept, ok := deferred[0].(*ast.AcceptEvent)
	if !ok {
		t.Fatalf("expected the first deferral to be a signal event, got %T", deferred[0])
	}
	if name := ast.SimpleName(accept.SignalType); name != "Ping" {
		t.Errorf("expected the deferred signal to be Ping, got %q", name)
	}
	call, ok := deferred[1].(*ast.CallEvent)
	if !ok {
		t.Fatalf("expected the second deferral to be a call event, got %T", deferred[1])
	}
	if name := ast.SimpleName(call.Operation); name != "setSpeed" {
		t.Errorf("expected the deferred call to be setSpeed, got %q", name)
	}
	if len(call.Parameters) != 1 || call.Parameters[0].Text != "value" {
		t.Errorf("expected the deferred call to carry the parameter value, got %v", call.Parameters)
	}
}

// A `defer` in the machine's own body has no state to defer for: the event
// would be retained for the whole run and never redelivered, so lowering
// reports it rather than dropping it.
func TestToStateGraph_DeferInMachineBodyIsReported(t *testing.T) {
	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				defer Ping;
				state busy;
				start then busy;
			}
		}
	`), nil)
	if err == nil {
		t.Fatal("expected an error for a defer in the state machine body")
	}
	if !strings.Contains(err.Error(), "defer must be declared inside a state") {
		t.Errorf("expected a placement error, got: %v", err)
	}
}

// A region is not a state either, so a defer in a region body is reported
// rather than silently dropped.
func TestToStateGraph_DeferInRegionBodyIsReported(t *testing.T) {
	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				state outer {
					region left {
						initial lstart;
						defer Ping;
						state lwork;
						then lstart lwork;
					}
					region right {
						initial rstart;
						state rwork;
						then rstart rwork;
					}
				}
			}
		}
	`), nil)
	if err == nil {
		t.Fatal("expected an error for a defer in a region body")
	}
	if !strings.Contains(err.Error(), "defer must be declared inside a state") {
		t.Errorf("expected a placement error, got: %v", err)
	}
}

// History, entry and exit point pseudostates written in the textual notation
// reach the graph with the kind their keyword names and with the composite
// state that declares them as their owner.
func TestToStateGraph_HistoryAndPointNotation(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				state outer {
					state a;
					history resume;
					deep history resumeDeep;
					shallow history resumeShallow;
					entry point into;
					exit point outOf;
				}
				start then outer;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	want := map[string]ast.PseudostateKind{
		"resume":        ast.PseudostateShallowHistory,
		"resumeDeep":    ast.PseudostateDeepHistory,
		"resumeShallow": ast.PseudostateShallowHistory,
		"into":          ast.PseudostateEntry,
		"outOf":         ast.PseudostateExit,
	}
	for name, kind := range want {
		ps, ok := graph.Pseudostates[name]
		if !ok {
			t.Errorf("pseudostate %q is not in the graph", name)
			continue
		}
		if ps.Kind != kind {
			t.Errorf("pseudostate %q: expected kind %v, got %v", name, kind, ps.Kind)
		}
		owner, ok := graph.PseudostateOwner[ps]
		if !ok || owner.Name != "outer" {
			t.Errorf("pseudostate %q: expected owner outer, got %v", name, owner)
		}
	}
}

// A succession out of the machine's own entry subaction is how standard
// notation names the state a machine starts in, so the target is the graph's
// initial state without an `initial` pseudostate being declared.
func TestToStateGraph_EntrySuccessionNamesInitialState(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				entry; then off;
				state off;
				state on;
				off then on;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if graph.Initial == nil {
		t.Fatal("expected the entry succession to name an initial state")
	}
	if graph.Initial.Name != "off" {
		t.Errorf("expected off to be the initial state, got %q", graph.Initial.Name)
	}
}
