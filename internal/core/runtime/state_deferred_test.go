package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// acceptTrigger builds the trigger of a transition or deferral that reacts to a
// signal. The machines below are built on the AST directly, the way the history
// tests are; the `defer` notation is covered by the conformance cases.
func acceptTrigger(signal string) *ast.AcceptEvent {
	return &ast.AcceptEvent{
		SignalType: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: signal}}},
	}
}

func triggeredTransition(source, target, signal string) *ast.TransitionMember {
	trans := transitionMember(source, target)
	trans.Trigger = acceptTrigger(signal)
	return trans
}

// deferringMachine is init → busy → ready → done, where busy handles Go and
// ready handles Ping. Whether busy defers Ping decides what a Ping arriving
// while busy is active does.
func deferringMachine(defers bool) *ast.Usage {
	busy := &ast.StateNode{Name: "busy"}
	if defers {
		busy.Defer = []ast.Node{acceptTrigger("Ping")}
	}

	return &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			busy,
			&ast.StateNode{Name: "ready"},
			&ast.StateNode{Name: "done", IsFinal: true},
			transitionMember("init", "busy"),
			triggeredTransition("busy", "ready", "Go"),
			triggeredTransition("ready", "done", "Ping"),
		},
	}
}

// A state defers an event no active transition handles, and the event is
// delivered once the machine reaches a state that no longer defers it.
func TestDeferredEventIsDeliveredAfterLeavingTheDeferringState(t *testing.T) {
	exec := stateExecutorFor(t, deferringMachine(true))
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "init", "busy", "ready", "done")
	if len(exec.deferred) != 0 {
		t.Errorf("expected no event still deferred, got %d", len(exec.deferred))
	}
}

// Without the deferral the same Ping is dropped where no transition handles it,
// which is what makes the test above evidence of deferral rather than of queue
// ordering.
func TestUndeferredEventIsDroppedWhereNoTransitionHandlesIt(t *testing.T) {
	exec := stateExecutorFor(t, deferringMachine(false))
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "init", "busy", "ready")
}

// A composite state's deferral holds while any of its substates is active: the
// event is retained although the state deferring it is not the active one.
func TestCompositeStateDefersForItsSubstates(t *testing.T) {
	inner := &ast.StateNode{Name: "inner"}
	outer := &ast.StateNode{
		Name:      "outer",
		Defer:     []ast.Node{acceptTrigger("Ping")},
		Substates: []ast.Node{inner},
	}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			outer,
			&ast.StateNode{Name: "ready"},
			&ast.StateNode{Name: "done", IsFinal: true},
			transitionMember("init", "inner"),
			triggeredTransition("inner", "ready", "Go"),
			triggeredTransition("ready", "done", "Ping"),
		},
	}

	exec := stateExecutorFor(t, machine)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !containsState(exec.stateVisits, "done") {
		t.Errorf("the Ping deferred by `outer` was not delivered after leaving it, visits: %v", exec.stateVisits)
	}
}

// Deferred events keep their arrival order: reversing them would leave the
// machine stuck in gotPing, because only gotPing handles Pong.
func TestDeferredEventsKeepTheirArrivalOrder(t *testing.T) {
	busy := &ast.StateNode{
		Name:  "busy",
		Defer: []ast.Node{acceptTrigger("Ping"), acceptTrigger("Pong")},
	}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			busy,
			&ast.StateNode{Name: "ready"},
			&ast.StateNode{Name: "gotPing"},
			&ast.StateNode{Name: "done", IsFinal: true},
			transitionMember("init", "busy"),
			triggeredTransition("busy", "ready", "Go"),
			triggeredTransition("ready", "gotPing", "Ping"),
			triggeredTransition("gotPing", "done", "Pong"),
		},
	}

	exec := stateExecutorFor(t, machine)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Pong", nil)
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "init", "busy", "ready", "gotPing", "done")
}

// orthogonalDeferMachine has two regions: `left` defers Ping and leaves its
// deferring state on Go, `right` handles Ping only if handlesPing.
func orthogonalDeferMachine(handlesPing bool) *ast.Usage {
	lwait := &ast.StateNode{Name: "lwait", Defer: []ast.Node{acceptTrigger("Ping")}}
	left := &ast.StateRegion{
		Name: "left",
		States: []ast.Node{
			&ast.StateNode{Name: "lstart", IsInitial: true},
			lwait,
			&ast.StateNode{Name: "lopen"},
			&ast.StateNode{Name: "lping"},
			transitionMember("lstart", "lwait"),
			triggeredTransition("lwait", "lopen", "Go"),
			triggeredTransition("lopen", "lping", "Ping"),
		},
	}

	rightStates := []ast.Node{
		&ast.StateNode{Name: "rstart", IsInitial: true},
		&ast.StateNode{Name: "rwait"},
		&ast.StateNode{Name: "rping"},
		transitionMember("rstart", "rwait"),
	}
	if handlesPing {
		rightStates = append(rightStates, triggeredTransition("rwait", "rping", "Ping"))
	}

	return &ast.Usage{
		Kind:    ast.UsageState,
		Ident:   ast.Identification{Name: "Machine"},
		Members: []ast.Node{left, &ast.StateRegion{Name: "right", States: rightStates}},
	}
}

// An event one region defers and no region handles is retained, and reaches the
// deferring region once it leaves the state that deferred it.
func TestDeferralSpansOrthogonalRegions(t *testing.T) {
	exec := stateExecutorFor(t, orthogonalDeferMachine(false))
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !containsState(exec.stateVisits, "lping") {
		t.Errorf("the deferred Ping was not delivered to the left region, visits: %v", exec.stateVisits)
	}
}

// An event another region consumes is not deferred: deferral only retains what
// the active configuration leaves unhandled.
func TestEventConsumedByAnotherRegionIsNotDeferred(t *testing.T) {
	exec := stateExecutorFor(t, orthogonalDeferMachine(true))
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !containsState(exec.stateVisits, "rping") {
		t.Errorf("the right region did not consume the Ping, visits: %v", exec.stateVisits)
	}
	if containsState(exec.stateVisits, "lping") {
		t.Errorf("a consumed event must not also be deferred, visits: %v", exec.stateVisits)
	}
	if len(exec.deferred) != 0 {
		t.Errorf("expected no event deferred, got %d", len(exec.deferred))
	}
}

// A transition whose guard is false does not consume its event, so a state that
// defers the event retains it rather than losing it to the blocked transition.
func TestEventBlockedByAGuardIsStillDeferred(t *testing.T) {
	blocked := triggeredTransition("busy", "wrong", "Ping")
	blocked.Guard = &ast.LiteralBool{Value: false}

	busy := &ast.StateNode{Name: "busy", Defer: []ast.Node{acceptTrigger("Ping")}}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			busy,
			&ast.StateNode{Name: "wrong"},
			&ast.StateNode{Name: "ready"},
			&ast.StateNode{Name: "done", IsFinal: true},
			transitionMember("init", "busy"),
			blocked,
			triggeredTransition("busy", "ready", "Go"),
			triggeredTransition("ready", "done", "Ping"),
		},
	}

	exec := stateExecutorFor(t, machine)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "init", "busy", "ready", "done")
}

// An event is broadcast to the regions active when it is dispatched: a region
// another region's transition just exited must not still react to it and come
// back to life. Here region a leaves the composite on Ping, so region b's own
// Ping transition never gets to fire.
func TestExitedNestedRegionDoesNotReactToTheSameEvent(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial start;
			state co {
				region a {
					initial astart;
					state a1;
					succession first astart then a1;
					transition a1 to out accept Ping;
				}
				region b {
					initial bstart;
					state b1;
					state b2;
					succession first bstart then b1;
					transition b1 to b2 accept Ping;
				}
			}
			state out;
			succession first start then co;
		}
	}`)

	exec.SendSignal("Ping", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if containsState(exec.stateVisits, "b2") {
		t.Errorf("the exited region took the event too, visits: %v", exec.stateVisits)
	}
	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions are still active after leaving co: %v", exec.activeConfig.regionStates)
	}
	current, _ := exec.CurrentState().(*ast.StateNode)
	if current == nil || current.Name != "out" {
		t.Errorf("expected the machine in out, got %v", exec.CurrentState())
	}
}

// A recalled event keeps its place ahead of the signals that arrived while it was
// held back: Ping arrived before Pong, so ready reacts to Ping although Pong was
// queued first.
func TestRecalledEventPrecedesLaterArrivals(t *testing.T) {
	busy := &ast.StateNode{Name: "busy", Defer: []ast.Node{acceptTrigger("Ping")}}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			busy,
			&ast.StateNode{Name: "ready"},
			&ast.StateNode{Name: "gotPing", IsFinal: true},
			&ast.StateNode{Name: "gotPong", IsFinal: true},
			transitionMember("init", "busy"),
			triggeredTransition("busy", "ready", "Go"),
			triggeredTransition("ready", "gotPing", "Ping"),
			triggeredTransition("ready", "gotPong", "Pong"),
		},
	}

	exec := stateExecutorFor(t, machine)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Ping", nil)
	exec.SendSignal("Go", nil)
	exec.SendSignal("Pong", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "init", "busy", "ready", "gotPing")
}
