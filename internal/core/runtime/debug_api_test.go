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

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}`

const debugStateSrc = `package test {
	state Cycle {
		entry; then init;
		state init;
		state waiting {
			accept after 10 then working;
		}
		state working {
			accept after 5 then done;
		}

		succession first init then waiting;
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
	for _, want := range []string{"start", "accumulate", "done"} {
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

// A step stating a short name and a redefinition answers to both keys, so a
// breakpoint may name either one.
func TestBreakpointNamesEitherKeyOfAShortNamedStep(t *testing.T) {
	const src = `package test {
	action def Base { action accumulate; }
	action tally : Base {
		attribute total = 0;
		first start;
		action <acc> :>> accumulate {
			assign total := total + 5;
		}
		done;
		succession first start then acc;
		succession first acc then done;
	}
}`

	for _, breakpoint := range []string{"acc", "accumulate"} {
		ctx, sym := loadAction(t, src, "tally")
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		exec.SetBreakpoint(breakpoint)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		if got := exec.PausedAt(); got != breakpoint {
			t.Errorf("PausedAt() = %q, want %s", got, breakpoint)
		}
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
	if exec.State() == StateSuspended {
		t.Error("Step() left the executor suspended")
	}
}

// blockDebugSrc declares action nodes inside an `if` branch and a loop body.
const blockDebugSrc = `package test {
	private import ScalarValues::*;
	action outer {
		attribute total : Integer = 0;
		first start;
		then action choose {
			if total == 0 {
				action p { out v : Integer = 7; }
				action q { in n : Integer = p.v; assign total := total + n; }
			}
		}
		then action iterate {
			for i in 1..3 {
				action add { in n : Integer = i; assign total := total + n; }
			}
		}
		then done;
	}
}`

// blockDebugExecutor builds an initialized executor for the outer action.
func blockDebugExecutor(t *testing.T) *ActionExecutor {
	t.Helper()
	ctx, sym := loadAction(t, blockDebugSrc, "outer")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	return exec
}

// A node a block declares is one a breakpoint can name.
func TestNodeNamesIncludeBlockFlowNodes(t *testing.T) {
	names := strings.Join(blockDebugExecutor(t).NodeNames(), ",")
	for _, want := range []string{"choose", "p", "q", "iterate", "add"} {
		if !strings.Contains(","+names+",", ","+want+",") {
			t.Errorf("NodeNames() = %s, want it to contain %q", names, want)
		}
	}
}

// A breakpoint on a node an `if` branch declares pauses the run before that node
// performs, with the branch's token left at the node running the block.
func TestBreakpointPausesBeforeABranchNode(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("q")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "q" {
		t.Fatalf("PausedAt() = %q, want q", got)
	}
	if got := exec.State(); got != StateSuspended {
		t.Errorf("State() = %v, want %v", got, StateSuspended)
	}
	if tokens := exec.Tokens(); len(tokens) != 1 || ActionNodeName(tokens[0].Location) != "choose" {
		t.Fatalf("expected one token at choose, got %v", tokens)
	}
	results := exec.Results()
	if v, ok := results["choose.p.v"]; !ok || v.Const.Int != 7 {
		t.Errorf("results = %v, want choose.p.v 7 (p performed before the pause)", results)
	}
	if _, ok := results["choose.q.n"]; ok {
		t.Errorf("results = %v, q must not have performed yet", results)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
	if total := exec.Results()["total"]; total.Const.Int != 13 {
		t.Errorf("total = %v, want 13", total)
	}
}

// A breakpoint on a node a loop body declares pauses the run once per iteration,
// resuming once per pause, as a breakpoint on a node of the action's own flow does.
func TestBreakpointPausesOnEachLoopIteration(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("add")

	for iteration, wantTotal := range []int64{7, 8, 10} {
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("iteration %d: RunToCompletion: %v", iteration, err)
		}
		if got := exec.PausedAt(); got != "add" {
			t.Fatalf("iteration %d: PausedAt() = %q, want add", iteration, got)
		}
		if total := exec.Results()["total"]; total.Const.Int != wantTotal {
			t.Errorf("iteration %d: total = %v, want %d", iteration, total, wantTotal)
		}
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("final run: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q after completing, want empty", got)
	}
	if total := exec.Results()["total"]; total.Const.Int != 13 {
		t.Errorf("total = %v, want 13", total)
	}
}

// A breakpoint on a step of the flow a block node owns pauses the run when a
// token of that flow reaches it, with the tokens of both flows in view.
func TestBreakpointPausesInsideABlockNodesOwnFlow(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
		private import ScalarValues::*;
		action outer {
			out attribute total : Integer = 0;
			first start;
			then action choose {
				if total == 0 {
					action split {
						out sum : Integer;
						first start;
						then action left { out a : Integer; assign a := 10; }
						then action gather { assign sum := left.a + 1; }
						then done;
					}
					action report { assign total := split.sum; }
				}
			}
			then done;
		}
	}`, "outer")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	exec.SetBreakpoint("gather")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "gather" {
		t.Fatalf("PausedAt() = %q, want gather", got)
	}
	var at []string
	for _, token := range exec.Tokens() {
		at = append(at, ActionNodeName(token.Location))
	}
	if got := strings.Join(at, ","); got != "choose,gather" {
		t.Errorf("tokens at %s, want choose,gather", got)
	}
	if a, ok := exec.Results()["choose.split.left.a"]; !ok || a.Const.Int != 10 {
		t.Errorf("results = %v, want choose.split.left.a 10", exec.Results())
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if total := exec.Results()["total"]; total.Const.Int != 11 {
		t.Errorf("total = %v, want 11", total)
	}
}

// Stepping resumes the paused block node and pauses again at the next one.
func TestStepResumesAPausedBlockNode(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("add")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}

	if err := exec.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := exec.PausedAt(); got != "add" {
		t.Errorf("PausedAt() = %q after a step, want the next iteration's add", got)
	}
	if total := exec.Results()["total"]; total.Const.Int != 8 {
		t.Errorf("total = %v after a step, want 8", total)
	}

	exec.ClearBreakpoints()
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
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
			entry; then init;
			state init;
			state working {
				do { count = count + 1; }
				accept after 100 then done;
			}
			succession first init then working;
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
		state def TrafficLight parallel {
			state pedestrian {
				entry; then start;
				state start;
				state Walk;
				succession first start then Walk;
			}
			state vehicle {
				entry; then begin;
				state begin;
				state Green;
				succession first begin then Green;
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
