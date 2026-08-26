package runtime

import "testing"

// A transition reaching `done` completes the machine: the state it leaves runs
// its exit action and the executor comes to rest completed.
func TestTransitionToDoneCompletesTheMachineAndRunsExitActions(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		private import ScalarValues::*;
		state sm {
			attribute left : Integer = 0;

			entry; then start;
			state start;
			state working {
				exit action { assign left := 1; }
			}
			succession first start then working;
			transition first working accept stop then done;
		}
	}`)

	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted, got %s", exec.State())
	}
	if got := exec.stateData["left"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the exit action of the completing state did not run, left = %v", got)
	}
}

// A machine whose states have no transition to `done` stays active, even where
// the state it rests in has no outgoing transition at all: a sink is not a
// completion, because an ancestor or cross-region transition may still leave it.
func TestMachineWithoutATransitionToDoneStaysActive(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state resting;
			succession first start then resting;
		}
	}`)

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "resting")
	if exec.State() == StateCompleted {
		t.Errorf("the machine completed without reaching `done`")
	}
}

// An orthogonal machine completes only once every top-level region has reached
// `done`; one region completing leaves the machine running.
func TestOrthogonalMachineCompletesOnlyWhenEveryRegionDoes(t *testing.T) {
	const src = `package test {
		state sm parallel {
			state left {
				entry; then lstart;
				state lstart;
				transition first lstart accept first then done;
			}
			state right {
				entry; then rstart;
				state rstart;
				transition first rstart accept second then done;
			}
		}
	}`

	exec := stateExecutorForSource(t, "sm", src)
	exec.SendSignal("first", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Fatalf("the machine completed with one region still running")
	}

	exec.SendSignal("second", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("expected StateCompleted once both regions reached `done`, got %s", exec.State())
	}
}

// The regions of a composite state complete the same way the machine's own do:
// one region reaching `done` leaves the machine running, and the machine
// completes once every region of that state has reached it.
func TestNestedOrthogonalRegionsCompleteOnlyWhenEveryRegionDoes(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then busy;
			state busy parallel {
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart accept first then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					transition first rstart accept second then done;
				}
			}
		}
	}`)

	exec.SendSignal("first", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Fatalf("the machine completed with one nested region still running")
	}

	exec.SendSignal("second", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("expected StateCompleted once both nested regions reached `done`, got %s", exec.State())
	}
}

// Completion is reached through a pseudostate as through any other path: the
// junction routes into `done` and the machine completes.
func TestCompletionReachedThroughAPseudostate(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state working;
			junction meet;
			succession first start then working;
			transition first working accept stop then meet;
			transition first meet then done;
		}
	}`)

	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("expected StateCompleted through the junction, got %s", exec.State())
	}
}

// A state the machine declares itself as `done` is an ordinary state and wins:
// completion is the library feature the name reaches when nothing nearer
// declares it, so a machine naming its own state `done` keeps running.
func TestADeclaredDoneStateIsAnOrdinaryState(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state done;
			transition first start accept stop then done;
		}
	}`)

	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if exec.State() == StateCompleted {
		t.Errorf("the declared state completed the machine")
	}
}
