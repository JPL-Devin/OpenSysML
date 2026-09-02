package repl

import (
	"slices"
	"strings"
	"testing"
)

// lampSession loads the signal-driven lamp and instantiates the bulb that
// exhibits it, leaving no debugging session open.
func lampSession(t *testing.T) *Session {
	t.Helper()
	s := loadFixture(t, "testdata/lamp_signals.sysml")
	wants(t, run(t, s, "%instantiate bulb"), "✓ Created instance of Lamps::bulb")
	return s
}

// TestSendDrivesAnAcceptTransition is the self-contained debugging loop the
// command exists for: attach, send, step, and the machine has moved.
func TestSendDrivesAnAcceptTransition(t *testing.T) {
	s := lampSession(t)
	wants(t, run(t, s, "%state bulb"), `✓ Debugging state machine "lamp"`, "Current state: off")
	wants(t, run(t, s, "%events"), "Event queue empty")

	wants(t, run(t, s, "%send go"),
		`✓ Sent go to object #1 of "Lamps::bulb"`,
		`Accepted by state machine "Lamp" in state off`,
		"Use %step or %advance <time> to dispatch it")
	wants(t, run(t, s, "%events"), "Signals in flight: 1", "  go")
	wants(t, run(t, s, "%step"), "Event dispatched")
	wants(t, run(t, s, "%current"), "Current state: on")
	wants(t, run(t, s, "%events"), "Event queue empty")
}

// TestSendCarriesNamedArguments binds the payload as %invoke binds parameters,
// and the accept's effect reads it.
func TestSendCarriesNamedArguments(t *testing.T) {
	s := lampSession(t)
	run(t, s, "%state bulb")
	run(t, s, "%send go")
	run(t, s, "%advance 1")
	wants(t, run(t, s, "%current"), "Current state: on")

	wants(t, run(t, s, "%send Dim(level = 2 + 5)"), "✓ Sent Dim(level=7)", `Accepted by state machine "Lamp" in state on`)
	wants(t, run(t, s, "%advance 1"), "Current state: dimmed")
	wants(t, run(t, s, "%features bulb"), "brightness", "7")
}

// TestSendToNamedObjectNeedsNoSession delivers to an object named after `to`,
// whose exhibited machine takes the signal on its own step.
func TestSendToNamedObjectNeedsNoSession(t *testing.T) {
	s := lampSession(t)
	wants(t, run(t, s, "%send go to bulb"), `✓ Sent go to object #1 of "Lamps::bulb"`, `Accepted by state machine "Lamp" in state off`)
	rejects(t, run(t, s, "%send Dim(level=1) to Lamps::bulb"), "✓ Sent")

	wants(t, run(t, s, "%state bulb"), "Current state: off")
	wants(t, run(t, s, "%events"), "Signals in flight: 1")
	wants(t, run(t, s, "%advance 1"), "Current state: on")
}

// TestSendResolvesQualifiedSignals accepts the signal's qualified name.
func TestSendResolvesQualifiedSignals(t *testing.T) {
	s := lampSession(t)
	run(t, s, "%state bulb")
	wants(t, run(t, s, "%send Lamps::go"), "✓ Sent go to object #1")
	wants(t, run(t, s, "%advance 1"), "Current state: on")
}

// TestSendRefusesWhatCannotBeDelivered covers every way a send goes wrong, each
// with a message that says what to do instead of a silently queued message.
func TestSendRefusesWhatCannotBeDelivered(t *testing.T) {
	s := lampSession(t)

	// No session and no `to`: nothing to guess a target from.
	wants(t, run(t, s, "%send go"), "error: no active state machine session", "name one with `to <object>`")
	wants(t, run(t, s, "%send"), "usage: %send <signal>")
	wants(t, run(t, s, "%send go bulb"), `unexpected "bulb" after the signal`, "usage: %send")
	wants(t, run(t, s, "%send go to"), "usage: %send")
	wants(t, run(t, s, "%send Dim(level=1 to bulb"), "not closed")

	// An object nothing runs a machine on, and one nothing instantiated.
	wants(t, run(t, s, "%send go to plain"), `error: no instance of "Lamps::plain"`, "%instantiate first")
	run(t, s, "%instantiate plain")
	wants(t, run(t, s, "%send go to plain"), `runs no state machine`, "%state <machine> <object>")
	wants(t, run(t, s, "%send go to nobody"), "error: unresolved reference: nobody")

	run(t, s, "%state bulb")
	// Unknown signals, in the wording every unresolved name gets, with a suggestion.
	wants(t, run(t, s, "%send gone"), "error: unresolved reference: gone", "did you mean")
	wants(t, run(t, s, "%send Lamps::gone"), "error: unresolved reference: Lamps::gone")
	// Not a signal at all.
	wants(t, run(t, s, "%send Lamp"), "error: Lamp is a state def, not a signal definition")

	// Arguments the signal has no feature for, malformed and doubled ones.
	wants(t, run(t, s, "%send go(level=1)"), "error: signal argument: go carries no feature", `go carries no feature "level"`, "(it carries none)")
	wants(t, run(t, s, "%send Dim(brightness=1)"), `Dim carries no feature "brightness"`, "(it carries level)")
	wants(t, run(t, s, "%send Dim(7)"), `argument "7" is not written as <parameter>=<expression>`)
	wants(t, run(t, s, "%send Dim(level=1, level=2)"), "error: argument level is given twice")
	wants(t, run(t, s, "%send Dim(level=nosuch)"), "error: argument level:")

	// A signal the machine does not take in its current state is refused, not
	// left in flight. The queue stays empty.
	wants(t, run(t, s, "%send Dim(level=1)"), `error: object #1 of "Lamps::bulb" accepts no signal Dim now`, `state machine "Lamp" in state off`)
	wants(t, run(t, s, "%send Reset"), "accepts no signal Reset now")
	wants(t, run(t, s, "%events"), "Event queue empty")
	wants(t, run(t, s, "%current"), "Current state: off")
}

// TestSendToAMachineNoObjectPerforms targets the executor a %state started for
// a definition alone.
func TestSendToAMachineNoObjectPerforms(t *testing.T) {
	s := loadFixture(t, "testdata/lamp_signals.sysml")
	wants(t, run(t, s, "%state Lamp"), `✓ Started state machine executor for "Lamp"`, "Current state: off")
	rejects(t, run(t, s, "%state Lamp"), "Performed by")
	wants(t, run(t, s, "%send go"), `✓ Sent go to state machine "Lamp"`, `Accepted by state machine "Lamp" in state off`)
	wants(t, run(t, s, "%advance 1"), "Current state: on")
}

// TestSendMatchesAnUndeclaredSignalByName: an accept naming a signal no
// declaration types is matched by name, as the runtime matches a model's send.
func TestSendMatchesAnUndeclaredSignalByName(t *testing.T) {
	s := loadFixture(t, "testdata/state_typed_usage.sysml")
	wants(t, run(t, s, "%state Machine"), "Current state: i1")
	wants(t, run(t, s, "%send Swop"), "error: unresolved reference: Swop")
	wants(t, run(t, s, "%send Swap(x=1)"), "error: unresolved reference: Swap")
	wants(t, run(t, s, "%send Swap"), `✓ Sent Swap to state machine "Machine"`, "No declaration types Swap, so the signal is matched by name alone")
	wants(t, run(t, s, "%advance 1"), "Current state: two")
}

// TestSendIsInHelpAndCompletion keeps the command discoverable.
func TestSendIsInHelpAndCompletion(t *testing.T) {
	if !strings.Contains(strings.Join(helpText(), "\n"), "%send") {
		t.Error("the send command is dispatched but not in help")
	}
	if !slices.Contains(metaCommands(), "%send") {
		t.Error("the send command is not in the command table")
	}
	s := lampSession(t)
	if got := s.Complete("%sen", len("%sen")); !slices.Contains(got.Candidates, "%send") {
		t.Errorf("completing %%sen offered %v, want %%send", got.Candidates)
	}
	if got := s.Complete("%send Di", len("%send Di")); !slices.Contains(got.Candidates, "Dim") {
		t.Errorf("completing a signal name offered %v, want Dim", got.Candidates)
	}
	if got := s.Complete("%send go to bu", len("%send go to bu")); !slices.Contains(got.Candidates, "bulb") {
		t.Errorf("completing the object offered %v, want bulb", got.Candidates)
	}
	wants(t, run(t, s, "%snd go"), "%send")
}

// TestStateOnAnObjectAttachesToItsRunningMachine: naming the definition and an
// object that already exhibits a machine of it debugs that machine, so the
// object never runs two.
func TestStateOnAnObjectAttachesToItsRunningMachine(t *testing.T) {
	s := lampSession(t)
	wants(t, run(t, s, "%state Lamp bulb"),
		`✓ Debugging state machine "lamp" exhibited by object #1 of "Lamps::bulb"`,
		"Attached to the machine already running rather than starting a second one",
		"Current state: off")
	rejects(t, run(t, s, "%state Lamp bulb"), "Started state machine executor")

	// It is the exhibited machine: a send moves the object's own state.
	run(t, s, "%send go")
	wants(t, run(t, s, "%advance 1"), "Current state: on")
	wants(t, run(t, s, "%stop"), "Stopped")
	wants(t, run(t, s, "%state bulb"), `✓ Debugging state machine "lamp"`, "Current state: on")

	// The usage form keeps attaching too.
	run(t, s, "%stop")
	wants(t, run(t, s, "%state Lamps::Bulb::lamp bulb"), `✓ Debugging state machine "lamp" exhibited by object #1`, "Attached to the machine already running")
}

// TestStateOnAnObjectStartsWhatItDoesNotRun: an object exhibiting no machine of
// the definition gets a fresh executor performed on its behalf, and says so.
func TestStateOnAnObjectStartsWhatItDoesNotRun(t *testing.T) {
	s := lampSession(t)
	run(t, s, "%instantiate plain")
	wants(t, run(t, s, "%state Lamp plain"),
		`✓ Started state machine executor for "Lamp"`,
		`of "Lamps::plain", which exhibits no running machine of this kind`,
		"Current state: off")
	wants(t, run(t, s, "%send go"), "✓ Sent go to object #", `of "Lamps::plain"`, `Accepted by state machine "Lamp" in state off`)
	wants(t, run(t, s, "%advance 1"), "Current state: on")
	// The bulb's own machine was not touched.
	run(t, s, "%stop")
	wants(t, run(t, s, "%state bulb"), "Current state: off")
}
