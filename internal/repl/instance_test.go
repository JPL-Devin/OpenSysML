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

// A requirement usage is a verdict too, and %slots must agree with what
// %requirement says about the same feature of the same instance.
func TestSlotsAgreesWithRequirementOnSameInstance(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")

	wants(t, run(t, s, "%requirement Derived::Vehicle::lightEnough"), "satisfied")
	got := run(t, s, "%slots Derived::Vehicle")
	wants(t, got, "lightEnough: <requirement: satisfied>")
	rejects(t, got, "<unknown>")
}

// A required condition that is false is the model's answer, not a malfunction,
// so it reports a verdict rather than an error — in both places that report it.
func TestRequirementViolationIsAVerdictNotAnError(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Heavy")

	got := run(t, s, "%requirement Derived::Heavy::lightEnough")
	wants(t, got, "✗ Requirement Derived::Heavy::lightEnough failed", "Required condition evaluated to false")
	rejects(t, got, "Error:")
	wants(t, run(t, s, "%slots Derived::Heavy"), "lightEnough: <requirement: violated>")
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
	if !hasNotice(res, `action debugging session for "tally" ended`) {
		t.Fatalf("notices = %v, want an ended-session note", res.Notices)
	}
	if !strings.Contains(strings.Join(renderResult(res, VerbosityNormal), "\n"), "ended") {
		t.Error("notice was not rendered to the user")
	}
	wants(t, run(t, s, "%step"), "no active action session")
}

// A behavior typed straight at the prompt owns itself, so redeclaring it ends
// the session the same way redeclaring its package does.
func TestTopLevelRedeclarationEndsDebugger(t *testing.T) {
	const tally = "action tally {\n\tattribute total = 0;\n\tfirst start;\n\taction accumulate {\n\t\tassign total := total + 5;\n\t}\n\tdone end;\n\tthen start accumulate;\n\tthen accumulate end;\n}"
	s := NewSession()
	if res := s.Submit(tally); len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	run(t, s, "%action tally")

	res := s.Submit(tally)
	if !hasNotice(res, `action debugging session for "tally" ended`) {
		t.Fatalf("notices = %v, want an ended-session note", res.Notices)
	}
	wants(t, run(t, s, "%step"), "no active action session")
}

// The same contract for the state machine debugger.
func TestStateDebuggerSurvivesUnrelatedSubmission(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	started := run(t, s, "%state Cycle")
	if !strings.Contains(started, "Started state machine executor") {
		t.Fatalf("%%state failed: %s", started)
	}

	if res := s.Submit(`package Unrelated { part def Widget { attribute size = 1.0; } }`); len(res.Notices) != 0 {
		t.Errorf("unrelated submission reported %v", res.Notices)
	}
	rejects(t, run(t, s, "%current"), "no active")
}

// The instances a submission invalidates are counted in a notice, so the
// objects created before it do not disappear without a word.
func TestSubmissionReportsTheInstancesItDropped(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Demo::Vehicle")
	run(t, s, "%instantiate Demo::Engine")

	res := s.Submit("part def Widget;")
	if !hasNotice(res, "2 instances were dropped") {
		t.Fatalf("notices = %v, want the dropped instances counted", res.Notices)
	}
	// A note reads as a consequence of the declaration it follows, so it comes
	// after the accepted line rather than before it.
	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	wants(t, out, "2 instances were dropped")
	if strings.Index(out, "✓ part def Widget") > strings.Index(out, "note:") {
		t.Errorf("note printed before the declaration it followed from:\n%s", out)
	}
}

// A command that would drive an ended action session says which submission
// ended it, instead of reporting only that nothing is active.
func TestEndedActionSessionExplainsItselfToEveryCommand(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	s.Submit("package Debug {\n\taction tally {\n\t\tfirst start;\n\t\tdone end;\n\t\tthen start end;\n\t}\n}")

	const why = `the action session for "tally" ended when Debug::tally was redeclared at submission 2`
	wants(t, run(t, s, "%step"), why)
	wants(t, run(t, s, "%tokens"), why)
	wants(t, run(t, s, "%continue"), why)
	wants(t, run(t, s, "%stop"), "ended when Debug::tally was redeclared at submission 2")

	// Starting a new session clears the explanation with it.
	run(t, s, "%action tally")
	rejects(t, run(t, s, "%step"), "no active action session")
}

// The same for the state machine debugger, whose commands are %current and
// %advance.
func TestEndedStateSessionExplainsItselfToEveryCommand(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	run(t, s, "%state Cycle")
	s.Submit("package Debug {\n\tstate Cycle {\n\t\tinitial init;\n\t\tfinal done;\n\t\tinit then done;\n\t}\n}")

	const why = `the state machine session for "Cycle" ended when Debug::Cycle was redeclared at submission 2`
	wants(t, run(t, s, "%current"), why)
	wants(t, run(t, s, "%advance 1"), why)
	wants(t, run(t, s, "%events"), why)
}

// A member of a nested part is answered against that part, not against the
// enclosing object that happens to be the instantiated one.
func TestNestedPartMemberBindsToTheNestedInstance(t *testing.T) {
	s := loadFixture(t, "testdata/nested_part.sysml")
	run(t, s, "%instantiate Nested::Car")

	got := run(t, s, "%eval Nested::Car::engine::mass")
	wants(t, got, "= 5.00")
	rejects(t, got, "on Nested::Car ID")
	wants(t, run(t, s, "%eval Nested::Car::mass"), "on Nested::Car ID", "= 1500.00")
	wants(t, run(t, s, "%constraint Nested::Car::engine::light"),
		"passed", "on Nested::Car::engine")
}

// A multi-valued feature shows what the object holds, not <unknown>.
func TestCollectionSlotsShowTheirContents(t *testing.T) {
	s := loadFixture(t, "testdata/collection_slots.sysml")
	run(t, s, "%instantiate Coll::Rig")

	got := run(t, s, "%slots Coll::Rig")
	wants(t, got, "doubles = [200.00]", "wheels = [Instance(ID: 2), Instance(ID: 3)]")
	rejects(t, got, "<unknown>")
	wants(t, run(t, s, "%eval Coll::Rig::doubles"), "= [200.00]")
}

// A part held in a slot is worth nothing to the reader as an opaque ID: %slots
// shows what the nested object holds, indented under the slot that holds it.
func TestSlotsExpandNestedInstances(t *testing.T) {
	s := loadFixture(t, "testdata/nested_part.sysml")
	run(t, s, "%instantiate Nested::Car")

	wants(t, run(t, s, "%slots Nested::Car"),
		"  engine = Instance(ID: 2)",
		"    mass = 5.00",
		"    light: <constraint: satisfied>",
	)
}

// Each element of a multi-valued part slot is expanded too.
func TestSlotsExpandCollectionElements(t *testing.T) {
	s := loadFixture(t, "testdata/collection_slots.sysml")
	run(t, s, "%instantiate Coll::Rig")

	got := run(t, s, "%slots Coll::Rig")
	wants(t, got, "wheels = [Instance(ID: 2), Instance(ID: 3)]")
	if strings.Count(got, "    radius") != 2 {
		t.Errorf("expected both wheels expanded, got:\n%s", got)
	}
}

// A part containing its own kind materializes a fresh object per expansion, so
// nesting is bounded by type rather than by instance identity.
func TestSlotsStopAtRecursiveContainment(t *testing.T) {
	s := NewSession()
	s.Submit("part def Node { attribute v = 1.0; part child : Node; }")
	run(t, s, "%instantiate Node")

	got := run(t, s, "%slots Node")
	wants(t, got, "v = 1.00", "child : Node (not expanded: contains its own kind)")
	if n := strings.Count(got, "\n"); n > 5 {
		t.Errorf("expected a bounded listing, got %d lines:\n%s", n, got)
	}
}

// Nesting multiplies, and every expansion materializes an object, so a wide
// model is truncated rather than listed in full.
func TestSlotsTruncateWideNesting(t *testing.T) {
	s := NewSession()
	s.Submit("part def Leaf { attribute v = 1.0; } part def Mid { part leaves : Leaf[20]; } part def Top { part mids : Mid[20]; }")
	run(t, s, "%instantiate Top")

	got := run(t, s, "%slots Top")
	wants(t, got, "… (listing truncated)")
	if n := strings.Count(got, "\n"); n > maxSlotLines+10 {
		t.Errorf("listing ran to %d lines, want it bounded near %d:\n%.400s", n, maxSlotLines, got)
	}
}

// Adding a member to a package leaves the rest of its body as it was, so a
// debugging session over another member of it keeps running.
func TestDebugSessionSurvivesAnAdditionToItsPackage(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	res := s.Submit("package Debug { part def Widget; }")

	if hasNotice(res, "debugging session") {
		t.Errorf("notices = %v, want the untouched session kept", res.Notices)
	}
	wants(t, run(t, s, "%tokens"), "Active tokens")
}
