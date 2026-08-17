package runtime

import (
	"strings"
	"testing"
)

// A machine whose only outgoing transition is a change trigger progresses under
// RunToCompletion: the condition is re-tested by the run itself, not by whatever
// external driver happens to poll.
func TestChangeTriggerRunsWithoutAnExternalPoll(t *testing.T) {
	outputs, visits, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			attribute log : Integer = 0;
			initial start;
			state waiting {
				accept when ready then done;
			}
			state done { entry { log = 1; } }
			start then waiting;
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := []string{"start", "waiting", "done"}; !equalStrings(visits, want) {
		t.Errorf("visits = %v, want %v", visits, want)
	}
	if log := outputs["log"]; log.Kind != ValConst || log.Const.Int != 1 {
		t.Errorf("log = %v, want 1: the change transition did not fire", log)
	}
}

// A change condition risen by a do behavior is taken by the step after it, which
// is what re-testing per micro-step buys: the run neither waits for an event nor
// misses the rise.
func TestChangeTriggerFiresOnRiseFromDoBehavior(t *testing.T) {
	_, visits, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			attribute count : Integer = 0;
			initial start;
			state counting {
				do action tick {
					assign count := count + 1;
					assign count := count + 1;
				}
				accept when count >= 2 then done;
			}
			state done;
			start then counting;
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := []string{"start", "counting", "done"}; !equalStrings(visits, want) {
		t.Errorf("visits = %v, want %v", visits, want)
	}
}

// A false change condition is not quiescence: the machine suspends, and says
// which condition it is waiting on rather than reporting silent completion.
func TestChangeTriggerFalseConditionIsReported(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			initial start;
			state waiting {
				accept when ready then done;
			}
			state done;
			start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v, want suspended", exec.State())
	}
	reason := exec.SuspendReason()
	if !strings.Contains(reason, "waiting on change condition") || !strings.Contains(reason, "condition is false") {
		t.Errorf("reason = %q, want the false change condition it waits on", reason)
	}
	if !strings.Contains(reason, "when ready") {
		t.Errorf("reason = %q, want the trigger as written", reason)
	}

	// The condition rising makes the transition due; the same run takes it.
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if reason := exec.SuspendReason(); !strings.HasPrefix(reason, "quiesced") {
		t.Errorf("reason = %q, want quiescence once no condition is watched", reason)
	}
}

// A machine watching no change condition reports quiescence, so the two cases a
// stalled machine can be in stay distinguishable.
func TestQuiescedMachineReportsNoChangeCondition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial start;
			state waiting {
				accept sig then done;
			}
			state done;
			start then waiting;
		}
		attribute def sig;
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	reason := exec.SuspendReason()
	if !strings.HasPrefix(reason, "quiesced") {
		t.Errorf("reason = %q, want quiescence", reason)
	}
}

// A condition that stays true fires its edge once: a self-transition on a
// lasting condition suspends instead of exhausting the event budget.
func TestChangeTriggerDoesNotRefireUnchangedCondition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			attribute laps : Integer = 0;
			initial start;
			state waiting;
			start then waiting;
			transition waiting to waiting accept when ready do assign laps := laps + 1;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v, want suspended", exec.State())
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 1 {
		t.Errorf("laps = %v, want 1: the edge fired other than once on the rise", laps)
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "has not changed since it fired") {
		t.Errorf("reason = %q, want the already-fired condition", reason)
	}

	// Observing the condition false re-arms the edge, so the next rise fires.
	exec.stateData["ready"] = boolValue(false)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun false: %v", err)
	}
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun true: %v", err)
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 2 {
		t.Errorf("laps = %v, want 2: the re-armed edge did not fire on the next rise", laps)
	}
}

// A guard blocking a risen condition consumes nothing: the transition stays
// armed and the guard is re-tested, and the wait names the guard.
func TestChangeTriggerGuardBlocks(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			attribute allowed : Boolean = false;
			initial start;
			state waiting;
			state done;
			start then waiting;
			transition waiting to done accept when ready if allowed;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "waiting")
	if reason := exec.SuspendReason(); !strings.Contains(reason, "guard is false") {
		t.Errorf("reason = %q, want the blocking guard", reason)
	}

	exec.stateData["allowed"] = boolValue(true)
	fired, err := exec.PollChangeEvents()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !fired {
		t.Fatal("poll fired nothing once the guard passed")
	}
	assertCurrentState(t, exec, "done")
}

// Polling costs no event budget of its own: a machine whose conditions stay
// false runs to suspension under a budget of one event.
func TestChangeTriggerPollingKeepsEventBudget(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			initial start;
			state waiting {
				do action tick { assign ready := ready; }
				accept when ready then done;
			}
			state done;
			start then waiting;
		}
	}`)
	exec.ctx.maxStateEvents = 1
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v, want suspended", exec.State())
	}
}

// A guard blocked by an earlier candidate's effect in the same poll stays armed:
// the fire path re-tests the guard, so latching such an edge would lose it for as
// long as its condition stays true.
func TestChangeTriggerKeepsAnEdgeBlockedDuringThePoll(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			attribute allowed : Boolean = true;
			attribute laps : Integer = 0;
			initial start;
			state Working {
				region first {
					initial f0;
					state fWait;
					state fDone;
					transition f0 to fWait;
					transition fWait to fDone accept when ready do assign allowed := false;
				}
				region second {
					initial s0;
					state sWait;
					state sDone { entry { laps = 1; } }
					transition s0 to sWait;
					transition sWait to sDone accept when ready if allowed;
				}
			}
			start then Working;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Both regions watch the condition, so raising it puts both transitions in
	// one poll and the first one's effect blocks the second one's guard.
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("raise condition: %v", err)
	}
	if !containsState(exec.stateVisits, "fDone") {
		t.Fatalf("the first region did not fire, visits: %v", exec.stateVisits)
	}
	if containsState(exec.stateVisits, "sDone") {
		t.Fatalf("the second region fired against a blocked guard, visits: %v", exec.stateVisits)
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "guard is false") {
		t.Errorf("reason = %q, want the guard blocked during the poll", reason)
	}

	// The blocked edge is still armed, so it fires once its guard passes even
	// though its condition never went false.
	exec.stateData["allowed"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 1 {
		t.Errorf("laps = %v, want 1: the blocked edge was lost", laps)
	}
}

// A change condition moving the machine out of a deferring state recalls the
// events that state held back: the run dispatches the deferred signal rather
// than declaring itself settled with the signal still held.
func TestChangeTriggerRecallsDeferredEvents(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		attribute def Ping;
		state Machine {
			attribute ready : Boolean = false;
			initial start;
			state busy {
				defer Ping;
				accept when ready then waiting;
			}
			state waiting {
				accept Ping then done;
			}
			state done;
			start then busy;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	exec.SendSignal("Ping", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("deliver Ping: %v", err)
	}
	assertCurrentState(t, exec, "busy")
	if len(exec.deferred) != 1 {
		t.Fatalf("deferred = %d, want the Ping held by busy", len(exec.deferred))
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "still deferred") {
		t.Errorf("reason = %q, want the deferred event it holds", reason)
	}

	// The condition rising leaves busy, so the Ping it deferred is dispatched by
	// the same run rather than stranded.
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if len(exec.deferred) != 0 {
		t.Errorf("deferred = %d, want the recalled Ping delivered", len(exec.deferred))
	}
}

// equalStrings compares two ordered lists of names.
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
