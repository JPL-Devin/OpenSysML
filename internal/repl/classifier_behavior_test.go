package repl

import (
	"strings"
	"testing"
)

// Materializing a part starts the machine its type exhibits, so %state binds the
// debugger to that object's machine rather than a detached run of the usage.
func TestStateDebugsTheMachineAnObjectExhibits(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	got := run(t, s, "%state Obj::Monitor")
	wants(t, got, "exhibited by object #", "modes", "Current state: idle")

	// The entry action of the state the machine settled in wrote the object's
	// own feature value.
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 1")
}

// %current, %events, %step and %advance drive the object's machine, which the
// machine's own timer left waiting when the object was materialized.
func TestObjectMachineDebuggingCommands(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")
	run(t, s, "%state Obj::Monitor")

	wants(t, run(t, s, "%current"), "idle")
	wants(t, run(t, s, "%events"), "1 events")
	wants(t, run(t, s, "%step"), "Event dispatched", "Current state: awake")
	wants(t, run(t, s, "%advance 5"), "time is now 15.00")
	wants(t, run(t, s, "%current"), "awake")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 11")
}

// A second %instantiate is a second object with its own machine, and says which
// object the name now denotes.
func TestSecondInstantiateIsAnotherObject(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	first := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))

	again := run(t, s, "%instantiate Obj::Monitor")
	wants(t, again, "now denotes this object", "object #"+first+" is no longer named", "behavior of its own")
	if second := objectIDIn(t, run(t, s, "%features Obj::Monitor")); second == first {
		t.Errorf("second %%instantiate reused object #%s:\n%s", first, again)
	}
}

// An operation of the object's type runs with the object as its performer, so
// what it writes is that object's feature value.
func TestInvokeRunsAnOperationOnTheObject(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy n=4"), "Invoked bumpBy on object #")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 5")
}

// %invoke reports its usage, an operation the type does not own, an argument
// naming no parameter and a parameter left unbound.
func TestInvokeReportsItsFailureModes(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")

	wants(t, run(t, s, "%invoke Obj::Monitor"), "usage: %invoke")
	wants(t, run(t, s, "%invoke Obj::Monitor missing"), "error:", "missing")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy"), "error:", "unbound parameter")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy other=1"), "error:", "unbound parameter")
	wants(t, run(t, s, "%invoke Obj::Monitor bumpBy n"), "error:", "<parameter>=<expression>")
}

// An unrelated declaration submitted while an object's machine is being debugged
// leaves the session on the same object, with its identity and its state kept.
func TestObjectMachineSurvivesAnUnrelatedDeclaration(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_machine.sysml")
	run(t, s, "%instantiate Obj::Monitor")
	started := run(t, s, "%state Obj::Monitor")
	id := objectIDIn(t, started)

	if res := s.Submit("package Other { part def Unrelated; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}

	wants(t, run(t, s, "%current"), "idle")
	wants(t, run(t, s, "%features Obj::Monitor"), "count = 1", "ID: "+id)
}

// objectIDIn reports the object identity a command's output names, written either
// as `#<n>` or as `ID: <n>`.
func objectIDIn(t *testing.T, out string) string {
	t.Helper()
	at, width := strings.Index(out, "#"), 1
	if named := strings.Index(out, "ID: "); named >= 0 && (at < 0 || named < at) {
		at, width = named, len("ID: ")
	}
	if at < 0 {
		t.Fatalf("output names no object identity:\n%s", out)
	}
	id := ""
	for _, r := range out[at+width:] {
		if r < '0' || r > '9' {
			break
		}
		id += string(r)
	}
	if id == "" {
		t.Fatalf("output names no object identity:\n%s", out)
	}
	return id
}
