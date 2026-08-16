package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// A transition out of a composite state is enabled while any of its substates is
// active, so the composite handles an event its active substate does not.
func TestCompositeStateHandlesEventItsSubstateDoesNot(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			initial start;
			state Working {
				state Step1;
				state Step2;
				transition Step1 to Step2 accept next;
			}
			state Done;
			start then Step1;
			transition Working to Done accept abort;
		}
	}`)

	exec.SendSignal("abort", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Done")
}

// A guard that is false does not consume the event: matching continues outward,
// so the composite state still handles it.
func TestFalseGuardInsideACompositeStateFallsOutward(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute ready : Boolean = false;

			initial start;
			state Working {
				state Step1;
				state Step2;
				transition Step1 to Step2 accept abort if ready;
			}
			state Done;
			start then Step1;
			transition Working to Done accept abort;
		}
	}`)

	exec.SendSignal("abort", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Done")
	if containsState(exec.stateVisits, "Step2") {
		t.Errorf("the blocked transition fired anyway, visits: %v", exec.stateVisits)
	}
}

// A transition out of a state between the active substate and the machine's root
// leaves only the states below it: the composite state enclosing its source stays
// active.
func TestTransitionOutOfAnIntermediateCompositeStateKeepsItsOwnerActive(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			initial start;
			state Outer {
				state Middle {
					state Inner;
				}
				state Recovered;
				transition Middle to Recovered accept abort;
			}
			start then Inner;
			transition Outer to Done accept shutdown;
			state Done;
		}
	}`)

	exec.SendSignal("abort", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Recovered")
	if !exec.inActiveConfiguration(stateNamed(t, exec, "Outer")) {
		t.Error("Outer was left although the transition's source was Middle")
	}
}

// Outer still handles the event only it accepts once its own substate moved: a
// composite state keeps reacting for whichever substate is active.
func TestOuterCompositeStateStillHandlesItsEventAfterASubstateMoved(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			initial start;
			state Outer {
				state Middle {
					state Inner;
				}
				state Recovered;
				transition Middle to Recovered accept abort;
			}
			start then Inner;
			transition Outer to Done accept shutdown;
			state Done;
		}
	}`)

	exec.SendSignal("abort", nil)
	exec.SendSignal("shutdown", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "start", "Outer", "Middle", "Inner", "Recovered", "Done")
}

// An event no state accepts at any level is dropped: the machine stays where it
// was, and the event does not reach an enclosing state that never declared it.
func TestEventNoLevelAcceptsLeavesTheConfigurationAlone(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			initial start;
			state Working {
				state Step1;
				state Step2;
				transition Step1 to Step2 accept next;
			}
			state Done;
			start then Step1;
			transition Working to Done accept abort;
		}
	}`)

	exec.SendSignal("unknown", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Step1")
	if len(exec.deferred) != 0 {
		t.Errorf("expected the event dropped rather than deferred, got %d held", len(exec.deferred))
	}
}

// Run-to-completion: a state entered while an event is dispatched does not react
// to that same event, even when it accepts it and lies outside the state that
// just moved.
func TestOneEventTakesOneTransitionPerActiveLeaf(t *testing.T) {
	source := `package test {
		state sm {
			initial start;
			state Working {
				state Step1;
			}
			state Idle;
			state Done;
			start then Working::Step1;
			transition Step1 to Idle accept e;
			transition Idle to Done accept e;
		}
	}`

	exec := stateExecutorForSource(t, "sm", source)
	exec.SendSignal("e", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "Idle")
	if containsState(exec.stateVisits, "Done") {
		t.Errorf("one event took two transitions, visits: %v", exec.stateVisits)
	}

	exec = stateExecutorForSource(t, "sm", source)
	exec.SendSignal("e", nil)
	exec.SendSignal("e", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "Done")
}

// Every leaf inside a composite state selects the same transition out of it, and
// one event still takes that transition once, re-entering both regions.
func TestOneEventTakesACompositesTransitionOnce(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute log : Integer = 0;

			initial start;
			state Working {
				region left {
					initial lstart;
					state l1;
					then lstart l1;
				}
				region right {
					initial rstart;
					state r1;
					then rstart r1;
				}
			}
			start then Working;
			transition Working to Working accept restart do assign log := log + 1;
		}
	}`)
	exec.SendSignal("restart", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := exec.StateData()["log"]; got.Const.Int != 1 {
		t.Errorf("one event ran the composite's effect %v times, want 1", got.Const.Int)
	}
	for _, state := range []string{"l1", "r1"} {
		if visits := countVisits(exec.stateVisits, state); visits != 2 {
			t.Errorf("state %s entered %d times, want 2 (initial entry and re-entry), visits: %v", state, visits, exec.stateVisits)
		}
	}
}

// A change condition on a composite state is watched while a substate is active.
// Polling is the driver of change triggers, so the test polls directly.
func TestChangeConditionOnACompositeStateFiresWhileASubstateIsActive(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute ready : Boolean = false;

			initial start;
			state Working {
				state Step1;
				accept when ready then Done;
			}
			state Done;
			start then Step1;
		}
	}`)

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := exec.pollChangeEvents(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	assertCurrentState(t, exec, "Step1")

	exec.stateData["ready"] = boolValue(true)
	if err := exec.pollChangeEvents(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	assertCurrentState(t, exec, "Done")
}

// An enabled transition inside one region does not suppress the transition an
// enclosing state offers a concurrent region: only the state a fired transition
// left is disabled for this event.
func TestOneRegionsInnerTransitionLeavesAConcurrentRegionsOwnTransition(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			initial start;
			state Working {
				region left {
					initial lstart;
					state l1;
					state l2;

					then lstart l1;
					transition l1 to l2 accept e;
				}

				region right {
					initial rstart;
					state r1;
					state r2;

					then rstart r1;
					transition r1 to r2 accept e;
				}
			}
			start then Working;
		}
	}`)

	exec.SendSignal("e", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertVisits(t, exec.stateVisits, "start", "Working", "lstart", "rstart", "l1", "r1", "l2", "r2")
}

func assertCurrentState(t *testing.T, exec *StateExecutor, want string) {
	t.Helper()
	current, _ := exec.CurrentState().(*ast.StateNode)
	if current == nil || current.Name != want {
		t.Errorf("expected the machine in %s, got %v", want, exec.CurrentState())
	}
}

func stateNamed(t *testing.T, exec *StateExecutor, name string) *ast.StateNode {
	t.Helper()
	for _, state := range exec.graph.States {
		if state.Name == name {
			return state
		}
	}
	t.Fatalf("state %s not found in the lowered graph", name)
	return nil
}
