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
