package repl

import (
	"strings"
	"testing"
)

// TestInstantiatedModelEvaluatesDerivedSlots is the "executable model" contract:
// after %instantiate, an attribute defined in terms of other features reports a
// value rather than <unknown>, including through a nested part.
func TestInstantiatedModelEvaluatesDerivedSlots(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")

	got := run(t, s, "%slots Derived::Vehicle")
	wants(t, got,
		"mass = 1500.00",    // declared constant
		"doubled = 3000.00", // derived from a sibling feature
		"total = 1770.00",   // derived through a nested part: 1500 + 300*0.9
	)
	rejects(t, got, "<unknown>")
}

// A derived value is reachable by %eval too, and reports which instance
// produced it.
func TestEvalReadsDerivedSlotOfInstance(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")

	wants(t, run(t, s, "%eval Derived::Vehicle::doubled"),
		"✓ Derived::Vehicle::doubled (on Derived::Vehicle ID: 1)",
		"= 3000.00",
	)
}

// Without an instance the same name still evaluates, using declared defaults,
// and says nothing about an instance.
func TestEvalWithoutInstanceUsesDeclaredDefault(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	got := run(t, s, "%eval Derived::Vehicle::mass")
	wants(t, got, "= 1500.00")
	rejects(t, got, "(on ")
}

// TestConstraintBindsToInstance: the same constraint text gives opposite
// verdicts on two instances, which is only possible if evaluation sees the
// instance's slots.
func TestConstraintBindsToInstance(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")
	run(t, s, "%instantiate Derived::Heavy")

	wants(t, run(t, s, "%constraint Derived::Vehicle::massOK"),
		"✓ Constraint Derived::Vehicle::massOK passed (on Derived::Vehicle ID: 1)")

	failed := run(t, s, "%constraint Derived::Heavy::massOK")
	wants(t, failed,
		"✗ Constraint Derived::Heavy::massOK failed (on Derived::Heavy ID:",
		"Assertion evaluated to false",
	)
	// A false assertion is the model's answer, not a malfunction.
	rejects(t, failed, "Error:")
}

// %slots renders a constraint feature as a verdict; a slot value would be
// meaningless for it.
func TestSlotsRendersConstraintVerdict(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")
	run(t, s, "%instantiate Derived::Heavy")

	wants(t, run(t, s, "%slots Derived::Vehicle"), "massOK: <constraint: satisfied>")
	wants(t, run(t, s, "%slots Derived::Heavy"), "massOK: <constraint: violated>")
}

// A constraint over a feature nothing declares is still an error, distinct from
// a violated assertion.
func TestConstraintEvaluationErrorIsNotAViolation(t *testing.T) {
	s := NewSession()
	s.Submit(`package Bad { constraint broken { assert nonexistent > 0; } }`)
	got := run(t, s, "%constraint Bad::broken")
	wants(t, got, "✗ Constraint Bad::broken failed", "Error:")
	rejects(t, got, "Assertion evaluated to false")
}

// --- Debugger session lifetime across submissions ---

// An unrelated declaration must not silently end an in-progress debugging
// session: the next %step still works.
func TestUnrelatedSubmissionKeepsDebuggerAlive(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")

	res := s.Submit(`package Unrelated { part def Widget { attribute size = 1.0; } }`)
	if len(res.Notices) != 0 {
		t.Errorf("unrelated submission reported %v", res.Notices)
	}
	wants(t, run(t, s, "%step"), "✓ Step complete")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}

// Redeclaring the declaration the debugged behavior lives under rewrites the
// graph being stepped, so the session ends — with a notice, rather than an
// unexplained failure on the next %step.
func TestRedeclarationEndsDebuggerWithNotice(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")

	res := s.Submit("package Debug {\n\taction tally {\n\t\tfirst start;\n\t\tdone end;\n\t\tthen start end;\n\t}\n}")
	if len(res.Notices) != 1 || !strings.Contains(res.Notices[0], `action debugging session for "tally" ended`) {
		t.Fatalf("notices = %v, want an ended-session note", res.Notices)
	}
	if !strings.Contains(strings.Join(renderResult(res), "\n"), "ended") {
		t.Error("notice was not rendered to the user")
	}
	wants(t, run(t, s, "%step"), "no active action session")
}

// The same contract for the state machine debugger.
func TestStateDebuggerSurvivesUnrelatedSubmission(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	started := run(t, s, "%state")
	if strings.Contains(started, "error") {
		t.Fatalf("%%state failed: %s", started)
	}

	if res := s.Submit(`package Unrelated { part def Widget { attribute size = 1.0; } }`); len(res.Notices) != 0 {
		t.Errorf("unrelated submission reported %v", res.Notices)
	}
	rejects(t, run(t, s, "%current"), "No active")
}
