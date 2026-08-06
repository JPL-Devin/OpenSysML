package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

const forkJoinMachine = `package test {
    state Machine {
        attribute leftRan : Integer = 0;
        attribute rightRan : Integer = 0;
        attribute merged : Integer = 0;

        initial init;
        state idle;
        state running {
            region left {
                initial lstart;
                state working { entry { leftRan = 1; } }
                then lstart working;
            }
            region right {
                initial rstart;
                state watching { entry { rightRan = 1; } }
                then rstart watching;
            }
        }
        fork split;
        join sync;
        final done;

        init then idle;
        transition idle to split;
        transition split to working;
        transition split to watching;
        transition working to sync;
        transition watching to sync;
        transition sync to done;
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
        initial init;
        state idle;
        state running {
            region left {
                initial lstart;
                state working;
                state alsoWorking;
                then lstart working;
            }
            region right {
                initial rstart;
                state watching;
                then rstart watching;
            }
        }
        fork split;
        final done;

        init then idle;
        transition idle to split;
        transition split to working;
        transition split to alsoWorking;
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
        initial init;
        state idle;
        state running {
            region left {
                initial lstart;
                state working;
                then lstart working;
            }
            region right {
                initial rstart;
                state watching;
                state stillWatching;
                then rstart watching;
                transition watching to stillWatching;
            }
        }
        fork split;
        join sync;
        final done;

        init then idle;
        transition idle to split;
        transition split to working;
        transition split to watching;
        transition working to sync;
        transition stillWatching to sync;
        transition sync to done;
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

// Entry and exit points have no textual notation, so they are built directly on
// the AST here; the executor must route through them like a junction.
func TestEntryAndExitPointPseudostates(t *testing.T) {
	init := &ast.StateNode{Name: "init", IsInitial: true}
	inner := &ast.StateNode{Name: "inner"}
	outer := &ast.StateNode{Name: "outer", Substates: []ast.Node{inner}}
	done := &ast.StateNode{Name: "done", IsFinal: true}
	entryPoint := &ast.PseudostateNode{Kind: ast.PseudostateEntry, Name: "in"}
	exitPoint := &ast.PseudostateNode{Kind: ast.PseudostateExit, Name: "out"}

	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
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
	})
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
