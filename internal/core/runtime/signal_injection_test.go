package runtime

import (
	"errors"
	"strings"
	"testing"
)

const lampSource = `
	private import ScalarValues::*;
	attribute def go;
	attribute def Dim { attribute level : Integer; }
	attribute def Batch { attribute levels : Integer[2..*]; }
	state def Lamp {
		attribute brightness : Integer = 0;
		entry; then off;
		state off;
		transition off_on first off accept go then on;
		state on;
		transition on_dim first on accept d : Dim do assign brightness := d.level then dimmed;
		state dimmed;
		transition dim_out first dimmed accept after 5 then off;
	}
	part def Bulb { exhibit state lamp : Lamp; }
	part def Plain;
	part plain : Plain;
`

// A message built from outside the model is typed by its definition, addressed
// to its object, and delivered by the same step a model's send is.
func TestSignalMessageDrivesTheExhibitedMachine(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
	root := idx.DocumentRoot("lamp.sysml")
	bulb, err := ctx.Instantiate(resolveSymbol(t, root, "Bulb"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	exec := behavior.State
	if exec.Performer() != bulb {
		t.Errorf("Performer() = %v, want the bulb", exec.Performer())
	}

	goSym := resolveSymbol(t, root, "go")
	msg, err := ctx.SignalMessage(goSym, nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	if msg.Signal != goSym || msg.SignalType != "go" || msg.Object != bulb.ID || msg.Delivery != DeliverObject {
		t.Errorf("message = %+v, want go typed by its definition and addressed to the bulb", msg)
	}
	if !exec.AcceptsMessage(msg) {
		t.Fatal("the lamp in off does not accept go")
	}
	ctx.PostMessage(msg)
	if !exec.HasPendingSignal() {
		t.Fatal("the posted message is not pending for the lamp")
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if got := activeLeaf(exec); got != "on" {
		t.Fatalf("state after go = %s, want on", got)
	}

	dimSym := resolveSymbol(t, root, "Dim")
	if _, err := ctx.SignalMessage(dimSym, map[string]Value{"brightness": integerValue(1)}, bulb); !errors.Is(err, ErrSignalArgument) {
		t.Errorf("an argument Dim carries no feature for gave %v, want ErrSignalArgument", err)
	} else if !strings.Contains(err.Error(), "(it carries level)") {
		t.Errorf("the argument error does not name what Dim carries: %v", err)
	}
	if _, err := ctx.SignalMessage(dimSym, map[string]Value{"level": NewStringValue("x")}, bulb); !errors.Is(err, ErrSignalArgument) || !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("a string for Dim.level gave %v, want ErrSignalArgument wrapping ErrTypeMismatch", err)
	}
	batchSym := resolveSymbol(t, root, "Batch")
	if _, err := ctx.SignalMessage(batchSym, map[string]Value{"levels": integerValue(1)}, bulb); !errors.Is(err, ErrSignalArgument) || !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("one value for Batch.levels[2..*] gave %v, want ErrSignalArgument wrapping ErrMultiplicityViolation", err)
	}
	if len(ctx.PendingMessages()) != 0 || activeLeaf(exec) != "on" {
		t.Errorf("a refused message left the bus or the machine changed: %d pending, state %s", len(ctx.PendingMessages()), activeLeaf(exec))
	}
	dim, err := ctx.SignalMessage(dimSym, map[string]Value{"level": integerValue(7)}, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(Dim): %v", err)
	}
	ctx.PostMessage(dim)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Dim): %v", err)
	}
	if got := activeLeaf(exec); got != "dimmed" {
		t.Fatalf("state after Dim = %s, want dimmed", got)
	}
	if got := FormatValue(exec.StateData()["brightness"]); got != "7" {
		t.Errorf("brightness = %s, want 7 written by the accept's effect", got)
	}

	// A timer is now set for later; a signal in flight is still dispatched
	// first, since it is due now.
	if exec.EventQueue().Len() != 1 {
		t.Fatalf("queue holds %d events, want the dim_out timer", exec.EventQueue().Len())
	}
	if _, err := ctx.SignalMessage(resolveSymbol(t, root, "plain"), nil, bulb); !errors.Is(err, ErrNotASignal) {
		t.Errorf("a usage as a signal gave %v, want ErrNotASignal", err)
	}
}

// A signal in flight is due at the current time, so stepping dispatches it
// before a timer the queue holds for later.
func TestProcessNextEventTakesAPendingSignalBeforeALaterTimer(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		state def Waiter {
			entry; then idle;
			state idle;
			transition first idle accept after 10 then late;
			transition first idle accept Kick then kicked;
			state late;
			state kicked;
		}
		attribute def Kick;
	`)
	ctx := NewContext(model, resolver, 10000)
	exec, err := ctx.CreateStateExecutorFor(resolveSymbol(t, root, "Waiter"), nil)
	if err != nil {
		t.Fatalf("CreateStateExecutorFor: %v", err)
	}
	if exec.EventQueue().Len() != 1 {
		t.Fatalf("queue holds %d events, want the timer", exec.EventQueue().Len())
	}
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "Kick"), nil, nil)
	if err != nil {
		t.Fatalf("SignalMessage: %v", err)
	}
	ctx.PostMessage(msg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if got := activeLeaf(exec); got != "kicked" {
		t.Errorf("state = %s, want kicked: the signal is due before the timer", got)
	}
	if exec.CurrentTime() != 0 {
		t.Errorf("time = %v, want 0: dispatching the signal advances no clock", exec.CurrentTime())
	}
}

// ExhibitedMachineOf finds the machine an object runs by its definition or its
// usage, and none on an object exhibiting no such machine.
func TestExhibitedMachineOf(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
	root := idx.DocumentRoot("lamp.sysml")
	bulb, err := ctx.Instantiate(resolveSymbol(t, root, "Bulb"))
	if err != nil {
		t.Fatalf("Instantiate Bulb: %v", err)
	}
	plain, err := ctx.Instantiate(resolveSymbol(t, root, "Plain"))
	if err != nil {
		t.Fatalf("Instantiate Plain: %v", err)
	}
	lampDef := resolveSymbol(t, root, "Lamp")

	byDef, ok := ctx.ExhibitedMachineOf(bulb, lampDef)
	if !ok || byDef.Name != "lamp" {
		t.Fatalf("ExhibitedMachineOf(bulb, Lamp) = %v, %v; want the lamp usage", byDef, ok)
	}
	if byUsage, ok := ctx.ExhibitedMachineOf(bulb, byDef.Symbol); !ok || byUsage != byDef {
		t.Errorf("ExhibitedMachineOf(bulb, Bulb::lamp) = %v, %v; want the same machine", byUsage, ok)
	}
	if _, ok := ctx.ExhibitedMachineOf(plain, lampDef); ok {
		t.Error("ExhibitedMachineOf(plain, Lamp) found a machine on an object exhibiting none")
	}
	if _, ok := ctx.ExhibitedMachineOf(nil, lampDef); ok {
		t.Error("ExhibitedMachineOf(nil, Lamp) found a machine")
	}
}

// activeLeaf names the innermost active state.
func activeLeaf(exec *StateExecutor) string {
	states := exec.ActiveStates()
	if len(states) == 0 {
		return ""
	}
	return states[len(states)-1].Name
}
