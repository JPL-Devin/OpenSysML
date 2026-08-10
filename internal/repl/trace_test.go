package repl

import (
	"strings"
	"testing"
)

// Tracing reports the execution behind a command's answer, and reports each
// command's own steps only.
func TestTracingReportsExecutionPerCommand(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	wants(t, run(t, s, "%trace on"), "trace: on")
	run(t, s, "%instantiate Derived::Vehicle")

	got := run(t, s, "%slots Derived::Vehicle")
	wants(t, got, "[trace] ", "eval feature mass", "doubled = 3000.00")

	if again := run(t, s, "%instances"); strings.Contains(again, "[trace] ") {
		t.Errorf("a later command replayed an earlier trace:\n%s", again)
	}
}

func TestTracingIsOffByDefaultAndSwitchable(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	wants(t, run(t, s, "%trace"), "trace: off")
	rejects(t, run(t, s, "%instantiate Derived::Vehicle"), "[trace] ")

	run(t, s, "%trace on")
	run(t, s, "%trace off")
	if s.Tracing() {
		t.Error("tracing stayed on")
	}
	rejects(t, run(t, s, "%slots Derived::Vehicle"), "[trace] ")
	wants(t, run(t, s, "%trace loud"), `error: unknown trace setting "loud"`)
}

// Turning tracing on mid-session reaches the runtime context and the executor
// that already exist.
func TestTracingAppliesToAnExistingSession(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	run(t, s, "%trace on")

	wants(t, run(t, s, "%step"), "[trace] step 1:")
}
