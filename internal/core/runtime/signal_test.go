package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
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
// not take a message addressed to a sibling. The accept suspends waiting for
// its own message, and since nothing else can post one the run deadlocks.
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
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// An accept whose type no in-flight message carries waits for its own type
// rather than binding the wrong message or silently continuing, and reports
// the type it is still waiting for when the wait can never end.
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
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
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

// A send through a port reaches the accept listening on the port connected to
// it: the connection, not the accept's name, is what addresses the message.
func TestSendViaPortReachesConnectedAccept(t *testing.T) {
	outputs, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort; }
			action reader accept msg : Integer via inPort;
			action recorder { assign got := msg; }
			done end;
			then start sender;
			then sender reader;
			then reader recorder;
			then recorder end;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 42)
}

// A port-routed message is only for the accept on the port it arrived at: an
// accept listening on no port does not take it, however well its type matches.
func TestPortRoutedMessageBypassesPortlessAccept(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort; }
			action reader accept msg : Integer;
			done end;
			then start sender;
			then sender reader;
			then reader end;
		}
	}`)
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock for an accept on no port, got: %v", err)
	}
}

// The converse: an accept listening on a port does not take an addressed
// message, which travelled over no connection.
func TestAddressedMessageBypassesPortAccept(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port inPort;
			first start;
			action sender { send 42 to reader; }
			action reader accept msg : Integer via inPort;
			done end;
			then start sender;
			then sender reader;
			then reader end;
		}
	}`)
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock for an accept on a port, got: %v", err)
	}
	if !strings.Contains(err.Error(), "via inPort") {
		t.Errorf("expected the awaited port in the message, got: %v", err)
	}
}

// A connection joins its ends without a direction, so a send through either end
// reaches the other: the accept listens on the end the send did not name.
func TestSendViaPortRoutesInEitherDirection(t *testing.T) {
	outputs, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			port left;
			port right;
			connect right to left;
			first start;
			action sender { send 7 via left; }
			action reader accept msg : Integer via right;
			action recorder { assign got := msg; }
			done end;
			then start sender;
			then sender reader;
			then reader recorder;
			then recorder end;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 7)
}

// A state machine's transitions accept signals, never ports, so a message routed
// to a port is not swallowed by a machine that would otherwise react to it.
func TestPortRoutedMessageDoesNotReachStateMachine(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send Ping via outPort; }
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
	if _, _, err := ctx.ExecuteStateWithEvents(stateSym, nil); err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].Port != "inPort" {
		t.Fatalf("expected the port-routed Ping still in flight, got %v", pending)
	}
}

// A call event fires only the transition triggered by the operation invoked,
// and only when the call carries every argument the trigger declares.
func TestCallEventMatchesOperationName(t *testing.T) {
	callTrigger := func(operation string, params ...string) *lower.Transition {
		declared := make([]ast.NameSegment, len(params))
		for i, param := range params {
			declared[i] = ast.NameSegment{Text: param}
		}
		trigger := &ast.CallEvent{Parameters: declared}
		if operation != "" {
			trigger.Operation = &ast.QualifiedName{Parts: []ast.NameSegment{{Text: operation}}}
		}
		return &lower.Transition{Trigger: trigger}
	}
	callEvent := func(operation string, args ...string) *Event {
		call := Call{Operation: operation, Args: make(map[string]Value, len(args))}
		for _, arg := range args {
			call.Args[arg] = Value{}
		}
		return &Event{Type: EventCall, Payload: call}
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
		{"declared parameter present", callTrigger("open", "angle"), callEvent("open", "angle"), true},
		{"declared parameter missing", callTrigger("open", "angle"), callEvent("open"), false},
		{"one declared parameter of several missing", callTrigger("open", "angle", "speed"), callEvent("open", "angle"), false},
		{"undeclared arguments ignored", callTrigger("open"), callEvent("open", "angle"), true},
	}
	for _, tc := range tests {
		if got := exec.matchesEvent(tc.trans, tc.event); got != tc.matches {
			t.Errorf("%s: matchesEvent = %v, want %v", tc.name, got, tc.matches)
		}
	}
}

// A call whose guard rejects it must leave the machine's data as it was: its
// arguments are bound only for the guard, and undone when nothing fires.
func TestRejectedCallLeavesNoArgumentsBehind(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state waiting;
			state moving;
			init then waiting;
			transition waiting to moving accept setSpeed(value) if value > 0;
		}
	}`)
	exec.InvokeOperation("setSpeed", map[string]Value{
		"value": {Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
	})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	if value, held := exec.stateData["value"]; held {
		t.Errorf("the rejected call's argument is still in the machine's data: %v", value)
	}
}

// An accept whose message has not arrived parks the token rather than failing:
// the action is suspended at that node, and the token records what it waits for.
func TestAcceptParksTokenUntilMessageArrives(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done end;
			then start reader;
			then reader recorder;
			then recorder end;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	// Step until the accept parks: the executor waits rather than erroring.
	for i := 0; i < 10 && exec.State() != StateWaiting; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if exec.State() != StateWaiting {
		t.Fatalf("expected the executor to be waiting, got %v", exec.State())
	}
	tokens := exec.Tokens()
	if len(tokens) != 1 || tokens[0].Wait == nil {
		t.Fatalf("expected one parked token, got %+v", tokens)
	}
	if tokens[0].Wait.ParamName != "n" || tokens[0].Wait.SignalType != "Integer" {
		t.Errorf("unexpected wait: %+v", *tokens[0].Wait)
	}
	// Step 1 moves the token off `start` onto the accept; step 2 finds nothing
	// it can take and parks it there.
	if tokens[0].Wait.Since != 2 {
		t.Errorf("expected the token to have parked at step 2, got %d", tokens[0].Wait.Since)
	}

	// A message posted from outside the action resumes it.
	ctx.PostMessage(Message{
		SignalType: "Integer",
		Target:     "reader",
		Payload: map[string]Value{
			"value": {Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 9}},
		},
	})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume after the message arrived: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected the resumed action to complete, got %v", exec.State())
	}
	assertIntOutput(t, exec.Results(), "got", 9)
}

// A parked token holds its place in the queue's matching order: an accept that
// suspended before a message it cannot take arrived still takes only its own.
func TestParkedAcceptTakesOnlyItsOwnMessage(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done end;
			then start reader;
			then reader recorder;
			then recorder end;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	for i := 0; i < 10 && exec.State() != StateWaiting; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	// A String arrives first and must be left in flight; the Integer resumes it.
	ctx.PostMessage(Message{SignalType: "String", Target: "reader", Payload: map[string]Value{
		"value": {Kind: ValString, Str: "not for you"},
	}})
	ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Payload: map[string]Value{
		"value": {Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 4}},
	}})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume after the message arrived: %v", err)
	}
	assertIntOutput(t, exec.Results(), "got", 4)

	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].SignalType != "String" {
		t.Fatalf("expected the String still in flight, got %v", pending)
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

// Routing honors the selection a variation is bound to: a message sent through a
// port reaches the ports the selected `variant interface`'s connection joins it
// to, and not the ones an unselected variant would (SysML v2 §7.20). A
// connection belonging to no variation always routes.
func TestRoutingHonorsTheSelectedVariantConnection(t *testing.T) {
	conns := []lower.Connection{
		{Ends: []string{"outPort", "inPort"}, Variation: "link", Variant: "direct"},
		{Ends: []string{"outPort", "bypass"}, Variation: "link", Variant: "indirect"},
		{Ends: []string{"outPort", "always"}},
	}
	for _, tt := range []struct {
		selected string
		want     []string
	}{
		{"", []string{"always"}},
		{"direct", []string{"inPort", "always"}},
		{"indirect", []string{"bypass", "always"}},
	} {
		_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test { }`))
		if tt.selected != "" {
			ctx.selectedVariants[variantSelection{variation: "link"}] = tt.selected
		}
		ctx.postVia(conns, Message{SignalType: "Ping"}, "outPort", nil)
		var got []string
		for _, msg := range ctx.PendingMessages() {
			got = append(got, msg.Port)
		}
		if len(got) != len(tt.want) {
			t.Fatalf("selection %q routed to %v, want %v", tt.selected, got, tt.want)
		}
		for i, port := range tt.want {
			if got[i] != port {
				t.Fatalf("selection %q routed to %v, want %v", tt.selected, got, tt.want)
			}
		}
	}
}

// Two objects of one type each selecting a different variant of one variation
// route over their own selection: a message a behavior of one object sends
// reaches the ports that object's selected connection joins, and no port of the
// other object's (SysML v2 §7.20).
func TestRoutingIsPerOwnerVariantSelection(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		port def P;
		part def Sys {
			port outPort : P;
			port inPort : P;
			port bypass : P;
			variation interface link {
				variant interface direct connect outPort to inPort;
				variant interface indirect connect outPort to bypass;
			}
		}
		part alpha : Sys { interface :>> link = link::direct; }
		part beta : Sys { interface :>> link = link::indirect; }
	}`))
	for usage, want := range map[string]string{"test::alpha": "inPort", "test::beta": "bypass"} {
		self, err := ctx.Instantiate(oneSymbol(t, idx, usage))
		if err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		ctx.postVia(nil, Message{SignalType: "Ping"}, "outPort", self)
		var got []string
		for _, msg := range ctx.PendingMessages() {
			if msg.Object != self.ID {
				t.Errorf("%s routed a message into object %d", usage, msg.Object)
			}
			got = append(got, msg.Port)
		}
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s routed to %v, want [%s]", usage, got, want)
		}
		ctx.messages = nil
	}
}
