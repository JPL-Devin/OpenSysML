package runtime

import (
	"strings"
	"testing"
)

const debugActionSrc = `package test {
	action tally {
		attribute total = 0;

		first start;

		action accumulate {
			assign total := total + 5;
		}

		done end;

		then start accumulate;
		then accumulate end;
	}
}`

const debugStateSrc = `package test {
	state Cycle {
		initial init;
		state waiting {
			accept after 10 then working;
		}
		state working {
			accept after 5 then done;
		}
		final done;

		init then waiting;
	}
}`

// debugActionExecutor builds an initialized executor for the tally action.
func debugActionExecutor(t *testing.T) *ActionExecutor {
	t.Helper()
	ctx, sym := loadAction(t, debugActionSrc, "tally")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	return exec
}

// The accessors a debugger reads between steps report the executor's state.
func TestActionExecutorDebugAccessors(t *testing.T) {
	exec := debugActionExecutor(t)

	if got := exec.ActionSymbol(); got == nil || got.Name != "tally" {
		t.Fatalf("ActionSymbol() = %v, want tally", got)
	}
	if got := exec.State(); got != StateRunning {
		t.Errorf("State() = %v, want %v", got, StateRunning)
	}

	tokens := exec.Tokens()
	if len(tokens) != 1 {
		t.Fatalf("Tokens() = %d tokens, want 1", len(tokens))
	}
	if name := ActionNodeName(tokens[0].Location); name != "start" {
		t.Errorf("token sits at %q, want start", name)
	}

	// Tokens is a copy: mutating it must not disturb the executor.
	tokens[0].ID = -1
	if exec.Tokens()[0].ID == -1 {
		t.Error("Tokens() exposed the executor's own slice")
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

func TestActionExecutorNodeNames(t *testing.T) {
	names := strings.Join(debugActionExecutor(t).NodeNames(), ",")
	for _, want := range []string{"start", "accumulate", "end"} {
		if !strings.Contains(names, want) {
			t.Errorf("NodeNames() = %s, want it to contain %q", names, want)
		}
	}
}

// A breakpoint stops a run when a token reaches the node, with the tokens left
// in place so the run can resume.
func TestSetBreakpointStopsRun(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("accumulate")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}

	if got := exec.PausedAt(); got != "accumulate" {
		t.Fatalf("PausedAt() = %q, want accumulate", got)
	}
	if got := exec.State(); got != StateSuspended {
		t.Errorf("State() = %v, want %v", got, StateSuspended)
	}
	tokens := exec.Tokens()
	if len(tokens) != 1 || ActionNodeName(tokens[0].Location) != "accumulate" {
		t.Fatalf("expected one token at accumulate, got %v", tokens)
	}

	// Resuming past the breakpoint completes the action.
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q after completing, want empty", got)
	}
	if total, ok := exec.Results()["total"]; !ok || total.Const.Int != 5 {
		t.Errorf("results = %v, want total 5", exec.Results())
	}
}

func TestClearBreakpointsResumesUnconditionally(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("accumulate")
	exec.ClearBreakpoints()

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q, want empty after clearing breakpoints", got)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

// A breakpoint on a node no token reaches leaves the run unaffected.
func TestBreakpointOnUnreachedNode(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("nowhere")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

// Stepping resumes a run a breakpoint suspended.
func TestStepResumesFromBreakpoint(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("accumulate")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}

	if err := exec.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := exec.State(); got == StateSuspended {
		t.Error("Step() left the executor suspended")
	}
}

func TestStateExecutorDebugAccessors(t *testing.T) {
	ctx, sym := loadState(t, debugStateSrc, "Cycle")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}

	if got := exec.StateMachineSymbol(); got == nil || got.Name != "Cycle" {
		t.Fatalf("StateMachineSymbol() = %v, want Cycle", got)
	}
	if got := exec.State(); got != StateRunning {
		t.Errorf("State() = %v, want %v", got, StateRunning)
	}
	if got := exec.CurrentTime(); got != 0 {
		t.Errorf("CurrentTime() = %v, want 0", got)
	}
	if exec.EventQueue().Len() == 0 {
		t.Error("EventQueue() is empty, want the initial completion event")
	}
	if got := activeStateNames(exec); got != "init" {
		t.Errorf("ActiveStates() = %s, want init", got)
	}
	if exec.StateData() == nil {
		t.Error("StateData() = nil")
	}

	// Drain the queue: time follows the events' timestamps.
	for exec.HasPendingWork() && exec.State() == StateRunning {
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent: %v", err)
		}
	}

	if got := exec.CurrentTime(); got != 15 {
		t.Errorf("CurrentTime() = %v, want 15", got)
	}
	if got := activeStateNames(exec); got != "done" {
		t.Errorf("ActiveStates() = %s, want done", got)
	}
	if len(exec.StateStack()) == 0 {
		t.Error("StateStack() is empty")
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

// RunDoRound advances a state's do behavior without dispatching an event, so a
// debugger can run work that is due now while leaving a future event queued.
func TestRunDoRoundRunsDoWorkOnly(t *testing.T) {
	exec := stateExecutorForSource(t, "Slow", `package test {
		state Slow {
			attribute count = 0;
			initial init;
			state working {
				do { count = count + 1; }
				accept after 100 then done;
			}
			final done;
			init then working;
		}
	}`)

	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if !exec.HasPendingDoWork() {
		t.Fatal("HasPendingDoWork() = false, want the do behavior of working pending")
	}

	queued := exec.EventQueue().Len()
	ran, err := exec.RunDoRound()
	if err != nil {
		t.Fatalf("RunDoRound: %v", err)
	}
	if ran != 1 {
		t.Errorf("RunDoRound() ran %d actions, want 1", ran)
	}
	if got := exec.EventQueue().Len(); got != queued {
		t.Errorf("event queue length = %d, want %d (no event dispatched)", got, queued)
	}
	if got := exec.CurrentTime(); got != 0 {
		t.Errorf("CurrentTime() = %v, want 0 (the future event is untouched)", got)
	}
	if exec.HasPendingDoWork() {
		t.Error("HasPendingDoWork() = true after the behavior's only action ran")
	}
}

// ActiveStates reports every region's state for an orthogonal machine, where
// CurrentState has no single answer to give.
func TestActiveStatesCoversOrthogonalRegions(t *testing.T) {
	exec := stateExecutorForSource(t, "TrafficLight", `package test {
		state def TrafficLight {
			region pedestrian {
				initial start;
				state Walk;
				then start Walk;
			}
			region vehicle {
				initial begin;
				state Green;
				then begin Green;
			}
		}
	}`)

	if exec.CurrentState() != nil {
		t.Error("CurrentState() should have no single answer for an orthogonal machine")
	}
	if got := len(exec.ActiveStates()); got != 2 {
		t.Fatalf("ActiveStates() returned %d states, want one per region", got)
	}
}

// activeStateNames joins the machine's active configuration for comparison.
func activeStateNames(exec *StateExecutor) string {
	names := make([]string, 0, 2)
	for _, state := range exec.ActiveStates() {
		names = append(names, state.Name)
	}
	return strings.Join(names, "|")
}
