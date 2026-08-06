package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
)

// executeActionSource executes the named action declared in src.
func executeActionSource(t *testing.T, name, src string) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	return ctx.ExecuteAction(sym)
}

// A send addresses a specific consumer: an accept action named by no send must
// not take a message addressed to a sibling.
func testSendReachesOnlyItsAddressee(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send 7 to wanted; }
			action other accept n : Integer;
			action reader { assign got := n; }
			done end;
			then start sender;
			then sender other;
			then other reader;
			then reader end;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: the message is addressed to `wanted`, not `other`")
	}
	if !errors.Is(err, ErrNoMatchingMessage) {
		t.Errorf("expected ErrNoMatchingMessage, got: %v", err)
	}
}

// An accept whose type no in-flight message carries reports rather than binding
// the wrong message or silently continuing.
func testAcceptOfUnsentTypeReports(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : String = "none";
			first start;
			action sender { send 7 to reader; }
			action reader accept text : String;
			done end;
			then start sender;
			then sender reader;
			then reader end;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: only an Integer was sent")
	}
	if !errors.Is(err, ErrNoMatchingMessage) {
		t.Errorf("expected ErrNoMatchingMessage, got: %v", err)
	}
	if !strings.Contains(err.Error(), "String") {
		t.Errorf("expected the accepted type in the message, got: %v", err)
	}
}

// The bus is context-wide: a message an action sends reaches a state machine
// executed later in the same context, which is why the two are not per-executor
// queues.
func TestActionMessageReachesStateMachine(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			first start;
			action sender { send Ping to Driver; }
			done end;
			then start sender;
			then sender end;
		}
		state Driver {
			initial init;
			state waiting;
			final done;
			init then waiting;
			transition waiting to done when Ping;
		}
	}`))
	root := idx.DocumentRoot("<test>")
	actionSym := findSymbolByName(root, "pipeline", ast.DefAction)
	stateSym := findSymbolByName(root, "Driver", ast.DefState)
	if actionSym == nil || stateSym == nil {
		t.Fatal("pipeline or Driver not found")
	}
	if _, err := ctx.ExecuteAction(actionSym); err != nil {
		t.Fatalf("execute action: %v", err)
	}
	_, visits, err := ctx.ExecuteStateWithEvents(stateSym, nil)
	if err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	assertVisits(t, visits, "init", "waiting", "done")
}

// A message nobody accepts stays in flight rather than being consumed by an
// unrelated accept: it must remain observable on the bus.
func TestUnacceptedMessageStaysPending(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender {
				send 3 to reader;
				send "spare" to nobody;
			}
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done end;
			then start sender;
			then sender reader;
			then reader recorder;
			then recorder end;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 3)

	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("expected 1 message still in flight, got %d: %v", len(pending), pending)
	}
	if pending[0].SignalType != "String" || pending[0].Target != "nobody" {
		t.Errorf("unexpected pending message: %+v", pending[0])
	}
}

// A send whose message names a type sends that type, carrying no payload, so a
// state machine transition triggered by that signal fires.
func TestSendOfNamedTypeReachesStateMachine(t *testing.T) {
	_, visits, err := executeStateSource(t, "Driver", `package P {
		item def Ping;
		state Driver {
			initial start;
			state waiting { entry { send Ping to Driver; } }
			final done;
			start then waiting;
			transition waiting to done when Ping;
		}
	}`)
	if err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	assertVisits(t, visits, "start", "waiting", "done")
}

// A signal no transition out of the active configuration accepts is not
// swallowed: the machine suspends and the message stays in flight.
func TestStateMachineLeavesForeignSignalPending(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		item def Pong;
		state Driver {
			initial start;
			state waiting { entry { send Pong to Driver; } }
			final done;
			start then waiting;
			transition waiting to done when Ping;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Driver", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Driver not found")
	}
	if _, _, err := ctx.ExecuteStateWithEvents(sym, nil); err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].SignalType != "Pong" {
		t.Fatalf("expected Pong still in flight, got %v", pending)
	}
}

// A call event fires only the transition triggered by the operation invoked.
// The triggers are hand-built because the notation for a call trigger is not
// parsed yet, so call events are only reachable programmatically.
func TestCallEventMatchesOperationName(t *testing.T) {
	callTrigger := func(operation string) *lower.Transition {
		trigger := &ast.CallEvent{}
		if operation != "" {
			trigger.Operation = &ast.QualifiedName{Parts: []ast.NameSegment{{Text: operation}}}
		}
		return &lower.Transition{Trigger: trigger}
	}
	callEvent := func(operation string) *Event {
		return &Event{Type: EventCall, Payload: Call{Operation: operation}}
	}

	exec := &StateExecutor{}
	tests := []struct {
		name    string
		trans   *lower.Transition
		event   *Event
		matches bool
	}{
		{"same operation", callTrigger("open"), callEvent("open"), true},
		{"other operation", callTrigger("open"), callEvent("close"), false},
		{"unnamed operation accepts any", callTrigger(""), callEvent("close"), true},
		{"accept trigger ignores calls", &lower.Transition{Trigger: &ast.AcceptEvent{}}, callEvent("open"), false},
	}
	for _, tc := range tests {
		if got := exec.matchesEvent(tc.trans, tc.event); got != tc.matches {
			t.Errorf("%s: matchesEvent = %v, want %v", tc.name, got, tc.matches)
		}
	}
}

func assertVisits(t *testing.T, visits []string, want ...string) {
	t.Helper()
	if len(visits) != len(want) {
		t.Fatalf("visits = %v, want %v", visits, want)
	}
	for i, name := range want {
		if visits[i] != name {
			t.Fatalf("visits = %v, want %v", visits, want)
		}
	}
}

func assertIntOutput(t *testing.T, outputs map[string]Value, name string, want int64) {
	t.Helper()
	value, ok := outputs[name]
	if !ok {
		t.Fatalf("output %q missing from %v", name, outputs)
	}
	if value.Const.Int != want {
		t.Errorf("%s = %v, want %d", name, value.Const.Int, want)
	}
}
