package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const forkJoinMachine = `package test {
    state Machine {
        attribute leftRan : Integer = 0;
        attribute rightRan : Integer = 0;
        attribute merged : Integer = 0;

        entry; then init;
        state init;
        state idle;
        state running {
            region left {
                entry; then lstart;
                state lstart;
                state working { entry { leftRan = 1; } }
                succession first lstart then working;
            }
            region right {
                entry; then rstart;
                state rstart;
                state watching { entry { rightRan = 1; } }
                succession first rstart then watching;
            }
        }
        fork split;
        join sync;
        final done;

        succession first init then idle;
        transition first idle then split;
        transition first split then working;
        transition first split then watching;
        transition first working then sync;
        transition first watching then sync;
        transition first sync then done;
    }
}`

// A fork makes one state active per orthogonal region; the join only releases
// once every branch has arrived.
func TestForkAndJoinPseudostates(t *testing.T) {
	ctx, machine := loadState(t, forkJoinMachine, "Machine")

	data, visited, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}

	for _, name := range []string{"leftRan", "rightRan"} {
		if got := intValue(t, data, name); got != 1 {
			t.Errorf("%s = %d, want 1 (fork branch entered)", name, got)
		}
	}
	for _, want := range []string{"working", "watching", "done"} {
		if !containsState(visited, want) {
			t.Errorf("state %q not visited, visits: %v", want, visited)
		}
	}
}

func TestForkBranchesMustBeInDistinctRegions(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    state Machine {
        entry; then init;
        state init;
        state idle;
        state running {
            region left {
                entry; then lstart;
                state lstart;
                state working;
                state alsoWorking;
                succession first lstart then working;
            }
            region right {
                entry; then rstart;
                state rstart;
                state watching;
                succession first rstart then watching;
            }
        }
        fork split;
        final done;

        succession first init then idle;
        transition first idle then split;
        transition first split then working;
        transition first split then alsoWorking;
    }
}`, "Machine")

	_, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err == nil || !strings.Contains(err.Error(), "same region") {
		t.Fatalf("error = %v, want branches in the same region to be rejected", err)
	}
}

func TestJoinWaitsForEveryBranch(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    state Machine {
        entry; then init;
        state init;
        state idle;
        state running {
            region left {
                entry; then lstart;
                state lstart;
                state working;
                succession first lstart then working;
            }
            region right {
                entry; then rstart;
                state rstart;
                state watching;
                state stillWatching;
                succession first rstart then watching;
                transition first watching then stillWatching;
            }
        }
        fork split;
        join sync;
        final done;

        succession first init then idle;
        transition first idle then split;
        transition first split then working;
        transition first split then watching;
        transition first working then sync;
        transition first stillWatching then sync;
        transition first sync then done;
    }
}`, "Machine")

	_, visited, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}
	// The left branch reaches the join first and must wait for the right branch
	// to move through stillWatching before done is entered.
	for _, want := range []string{"working", "watching", "stillWatching", "done"} {
		if !containsState(visited, want) {
			t.Fatalf("state %q not visited, visits: %v", want, visited)
		}
	}
	if indexOfState(visited, "stillWatching") > indexOfState(visited, "done") {
		t.Errorf("join released before the right branch arrived, visits: %v", visited)
	}
}

// The executor must route through an entry or exit point like a junction. The
// machine is built on the AST directly; the `entry point`/`exit point` notation
// is covered by the state_entry_exit_points conformance case.
func TestEntryAndExitPointPseudostates(t *testing.T) {
	init := &ast.StateNode{Name: "init"}
	inner := &ast.StateNode{Name: "inner"}
	outer := &ast.StateNode{Name: "outer", Substates: []ast.Node{inner}}
	done := &ast.StateNode{Name: "done", IsFinal: true}
	entryPoint := &ast.PseudostateNode{Kind: ast.PseudostateEntry, Name: "in"}
	exitPoint := &ast.PseudostateNode{Kind: ast.PseudostateExit, Name: "out"}

	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			init, outer, done, entryPoint, exitPoint,
			transitionMember("init", "in"),
			transitionMember("in", "inner"),
			transitionMember("inner", "out"),
			transitionMember("out", "done"),
		},
	}

	exec := stateExecutorFor(t, machine)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	for exec.eventQueue.Len() > 0 && exec.state == StateRunning {
		if err := exec.processNextEvent(); err != nil {
			t.Fatalf("processNextEvent: %v", err)
		}
	}

	for _, want := range []string{"outer", "inner", "done"} {
		if !containsState(exec.stateVisits, want) {
			t.Errorf("state %q not visited, visits: %v", want, exec.stateVisits)
		}
	}
}

// entryStart designates the state a machine or region starts in, as the
// `entry; then <name>;` succession out of the body's entry action does.
func entryStart(name string) *ast.SuccessionEdge {
	return &ast.SuccessionEdge{
		SourceMember: &ast.EntryMember{},
		Target:       &ast.QualifiedName{Parts: []ast.NameSegment{{Text: name}}},
	}
}

func transitionMember(source, target string) *ast.TransitionMember {
	return &ast.TransitionMember{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: source}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: target}}},
	}
}

func stateExecutorFor(t *testing.T, machine *ast.Usage) *StateExecutor {
	t.Helper()
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 100000)

	exec, err := newStateExecutor(ctx, &symbols.Symbol{
		Kind: symbols.SymbolStateUsage,
		Name: machine.Ident.Name,
		Decl: machine,
	}, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	return exec
}

func containsState(visits []string, name string) bool {
	return indexOfState(visits, name) >= 0
}

func indexOfState(visits []string, name string) int {
	for i, visit := range visits {
		if visit == name {
			return i
		}
	}
	return -1
}
