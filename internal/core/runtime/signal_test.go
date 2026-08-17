package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
		if err := ctx.postVia(conns, Message{SignalType: "Ping"}, lower.Send{Target: "outPort", IsVia: true}, nil); err != nil {
			t.Fatalf("selection %q: %v", tt.selected, err)
		}
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
		if err := ctx.postVia(nil, Message{SignalType: "Ping"}, lower.Send{Target: "outPort", IsVia: true}, self); err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
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

// A `send … to r` naming a receiving node is for the object performing the
// sending behavior, not another object declaring the same node.
func TestAddressedSendStaysWithinTheSendingObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		item def Ping;
		part def Node {
			action listen {
				first start;
				action sender { send Ping() to reader; }
				action reader accept ping : Ping;
				done end;
				then start sender;
				then sender reader;
				then reader end;
			}
		}
		part alpha : Node;
		part beta : Node;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	send := lower.Send{Target: "reader", Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, alpha); err != nil {
		t.Fatalf("post: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("posted %d messages, want 1", len(pending))
	}
	if got := pending[0]; !got.reachedObject(alpha.ID) || got.reachedObject(beta.ID) {
		t.Errorf("message %+v is not confined to the sending object %d", got, alpha.ID)
	}
}

// An addressed target resolves through the instance graph: `alpha.inPort` names
// that object's port, not a same-named port of the sender.
func TestAddressedSendResolvesPortOfNamedObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Node {
			port inPort : PingPort;
			action listen { first start; done end; then start end; }
		}
		part alpha : Node;
		part beta : Node;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	send := lower.Send{Target: "alpha.inPort", TargetPath: true, Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, beta); err != nil {
		t.Fatalf("post: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("posted %d messages, want 1", len(pending))
	}
	got := pending[0]
	if got.Port != "inPort" || got.Object != alpha.ID {
		t.Errorf("addressed alpha.inPort delivered as %+v, want port inPort of object %d", got, alpha.ID)
	}
	if got.reachedObject(beta.ID) {
		t.Errorf("message %+v reached the sending object %d, which owns a same-named port", got, beta.ID)
	}
}

// A target descends composite features to the object owning the port: the
// addressee of `inner.inPort` is that part of the sending object, not the sender.
func TestAddressedSendDescendsToNestedPort(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Leaf { port inPort : PingPort; }
		part def Node {
			part inner : Leaf;
			action listen { first start; done end; then start end; }
		}
		part alpha : Node;
	}`))
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send := lower.Send{Target: "inner.inPort", TargetPath: true, Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, alpha); err != nil {
		t.Fatalf("post: %v", err)
	}
	inner, ok := ctx.slotObject(alpha, "inner")
	if !ok {
		t.Fatal("part inner materialized no object")
	}
	got := ctx.PendingMessages()[0]
	if got.Port != "inPort" || got.Object != inner.ID {
		t.Errorf("addressed inner.inPort delivered as %+v, want port inPort of object %d", got, inner.ID)
	}
}

// A target reaching no port of an addressable object is a typed error, not a
// delivery to whatever else carries the last segment's name.
func TestAddressedSendToUnreachablePortIsTyped(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		import ScalarValues::*;
		item def Ping;
		part def Node {
			attribute count : Integer = 0;
			action listen { first start; done end; then start end; }
		}
		part alpha : Node;
	}`))
	send := lower.Send{Target: "alpha.count", TargetPath: true, Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	err := ctx.post(nil, Message{SignalType: "Ping"}, send, instanceOfUsage(t, ctx, idx, "test::alpha"))
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("post to alpha.count: %v, want ErrUnroutableSend", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("an unroutable send posted %+v", ctx.PendingMessages())
	}
}

// A receiver named by a qualified name is the element the name resolves to, not
// a path through the sender's features: `::` separates namespaces, `.` chains
// features.
func TestAddressedSendToQualifiedNameReachesReceiver(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			first start;
			action sender { send Ping to P::Driver; }
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
	if _, err := ctx.ExecuteAction(findSymbolByName(root, "pipeline", ast.DefAction)); err != nil {
		t.Fatalf("execute action: %v", err)
	}
	_, visits, err := ctx.ExecuteStateWithEvents(findSymbolByName(root, "Driver", ast.DefState), nil)
	if err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	assertVisits(t, visits, "init", "waiting", "done")
}

// A qualified name addresses the element it resolves to, so a same-named feature
// of the sending object is not the addressee.
func TestAddressedSendToQualifiedNameSkipsSameNamedFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		import ScalarValues::*;
		package Other {
			action reader accept n : Integer;
		}
		part def Node {
			action reader accept n : Integer;
			action listen { first start; done end; then start end; }
		}
		part alpha : Node;
	}`))
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send := lower.Send{Target: "Other::reader", Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Integer"}, send, alpha); err != nil {
		t.Fatalf("post: %v", err)
	}
	got := ctx.PendingMessages()[0]
	if got.Object == alpha.ID {
		t.Errorf("`send to Other::reader` delivered as %+v, want no delivery to the sender's own reader", got)
	}
}

// A port a qualified name resolves to that the sender owns no occurrence of is
// unroutable, not a delivery to the sender's same-named port.
func TestAddressedSendToQualifiedPortOfAnotherTypeIsTyped(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Other { port inPort : PingPort; }
		part def Node {
			port inPort : PingPort;
			action listen { first start; done end; then start end; }
		}
		part alpha : Node;
	}`))
	send := lower.Send{Target: "Other::inPort", Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	err := ctx.post(nil, Message{SignalType: "Ping"}, send, instanceOfUsage(t, ctx, idx, "test::alpha"))
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("post to Other::inPort: %v, want ErrUnroutableSend", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("an unroutable send posted %+v", ctx.PendingMessages())
	}
}

// A namespace qualifies the object a path starts from, so `test::alpha.inPort`
// reaches alpha's port rather than being read as a port of the sender.
func TestAddressedSendThroughNamespaceQualifiedPathReachesObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Node {
			port inPort : PingPort;
			action listen { first start; done end; then start end; }
		}
		part alpha : Node;
		part beta : Node;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	send := lower.Send{
		Target:     "test.alpha.inPort",
		TargetPath: true,
		Scope:      declScope(oneSymbol(t, idx, "test::Node::listen")),
	}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, beta); err != nil {
		t.Fatalf("post: %v", err)
	}
	got := ctx.PendingMessages()[0]
	if got.Port != "inPort" || got.Object != alpha.ID {
		t.Errorf("addressed test::alpha.inPort delivered as %+v, want port inPort of object %d", got, alpha.ID)
	}
}

// A path led by a part naming several occurrences reaches no one object, so it
// is unroutable rather than attributed to the sending object.
func TestAddressedSendThroughMultiplePartIsTyped(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Leaf { port inPort : PingPort; }
		part def Node {
			part nodes : Leaf[3];
			action listen { first start; done end; then start end; }
		}
		part alpha : Node;
	}`))
	send := lower.Send{
		Target:     "nodes.inPort",
		TargetPath: true,
		Scope:      declScope(oneSymbol(t, idx, "test::Node::listen")),
	}
	err := ctx.post(nil, Message{SignalType: "Ping"}, send, nil)
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("post to nodes.inPort: %v, want ErrUnroutableSend", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("an unroutable send posted %+v", ctx.PendingMessages())
	}
}

// An object's behavior addresses a receiving node of an action it performs: the
// performance is no object of its own, so identity must not exclude it.
func TestAddressedSendReachesPerformedAction(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		import ScalarValues::*;
		action listener {
			first start;
			action reader accept n : Integer;
			done fin;
			then start reader;
			then reader fin;
		}
		part def Node {
			action main {
				first start;
				action sender { send 7 to reader; }
				perform listener;
				done fin;
				then start sender;
				then sender listener;
				then listener fin;
			}
		}
		part solo : Node;
	}`))
	main := oneSymbol(t, idx, "test::Node::main")
	solo := instanceOfUsage(t, ctx, idx, "test::solo")
	if _, err := ctx.ExecuteActionPerformedBy(main, solo, nil); err != nil {
		t.Fatalf("performed by an object: %v", err)
	}
}

// Confinement discriminates between objects, and only between objects: a
// behavior no object performs has no identity to exclude, either side of a send.
func TestConfinementCrossesBehaviorPerformedByNoObject(t *testing.T) {
	tests := []struct {
		name    string
		msg     Message
		object  int64
		reached bool
	}{
		{"same object", Message{Object: 1, Confined: true}, 1, true},
		{"other object", Message{Object: 1, Confined: true}, 2, false},
		{"unconfined", Message{Object: 1}, 2, true},
		{"object to no object", Message{Object: 1, Confined: true}, 0, true},
		{"no object to object", Message{Confined: true}, 1, true},
		{"port of another object", Message{Object: 1, Port: "inPort", Confined: true}, 0, false},
		{"port to another object", Message{Port: "inPort", Confined: true}, 1, false},
	}
	for _, tc := range tests {
		if got := tc.msg.reachedObject(tc.object); got != tc.reached {
			t.Errorf("%s: %+v reachedObject(%d) = %v, want %v", tc.name, tc.msg, tc.object, got, tc.reached)
		}
	}
}

// instanceOfUsage materializes the object a part usage occurs as, which is what
// an address naming that usage resolves to.
func instanceOfUsage(t *testing.T, ctx *Context, idx *symbols.Index, fqn string) *Instance {
	t.Helper()
	inst, err := ctx.occurrenceOf(oneSymbol(t, idx, fqn))
	if err != nil {
		t.Fatalf("instantiate %s: %v", fqn, err)
	}
	return inst
}
