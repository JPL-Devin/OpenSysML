package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
)

// joinLines is a command's output as one string, to assert fragments against.
func joinLines(out []string) string { return strings.Join(out, "\n") }

// A pinned context is the namespace the expression is evaluated in, so its
// members are named without qualification however the prompt's own scope moved.
func TestEvalInNamespaceReadsItsMembers(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	// A scratch package moves the prompt's default scope; the pinned one is unmoved.
	s.Submit("package Scratch { attribute q = 1; }")
	got := run(t, s, "%eval in Demo::Vehicle : mass * 2")
	wants(t, got, "mass * 2 (in Demo::Vehicle)", "= 3000")
	rejects(t, got, "unresolved reference")
}

// A context may name the package rather than the part, in which case its members
// are reached the way that package reaches them.
func TestEvalInPackageResolvesThroughIt(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%eval in Demo : Vehicle::mass"), "= 1500")
}

// Pinned to an object, the expression reads the values that object holds, as an
// unpinned %eval does after %instantiate.
func TestEvalInInstanceReadsItsFeatureValues(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Demo::Vehicle")
	got := run(t, s, "%eval in Demo::Vehicle : mass + 1.0")
	wants(t, got, "(on Demo::Vehicle ID: 1)", "= 1501")
}

// Naming a context does not change what an unpinned %eval means.
func TestEvalWithoutContextIsUnchanged(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%eval 1 + 1"), "= 2")
	wants(t, run(t, s, "%eval Demo::Vehicle::mass"), "= 1500")
}

// A context nothing declares is reported as the unresolved name it is, not as a
// failure of the expression.
func TestEvalInUnknownContextIsReported(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%eval in Nope::Nothing : 1 + 1"), "error:", "Nope::Nothing")
}

// The form itself is checked: a missing colon, a missing name or a missing
// expression is answered with the usage, never a panic or a hang.
func TestEvalInMalformedFormsReportUsage(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	for _, line := range []string{
		"%eval in Demo::Vehicle mass",
		"%eval in : 1 + 1",
		"%eval in Demo::Vehicle :",
		"%eval in",
	} {
		out, quit, err := s.RunMeta(line)
		if err != nil || quit {
			t.Fatalf("%s: err = %v, quit = %v", line, err, quit)
		}
		wants(t, joinLines(out), evalUsage)
	}
}

// "in" is a context only where a context can be named: an expression whose own
// text starts with a name spelled "in" is still an expression.
func TestEvalInIsNotTakenFromAnExpression(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	// No colon follows, so this is the expression `inx`, not a pinned context.
	out, _, err := s.RunMeta("%eval inx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rejects(t, joinLines(out), evalUsage)
}

// An expression holding a colon of its own — a name qualifier — is not read as
// the separator that pins a context.
func TestEvalQualifiedNameIsNotAContextSeparator(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%eval Demo::Vehicle::mass * 2"), "= 3000")
}

// A pinned evaluation that cannot be carried out reports why rather than
// panicking: the expression is broken, or the context holds no such name.
func TestEvalInFailuresAreTypedNotPanics(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	for _, line := range []string{
		"%eval in Demo::Vehicle : mass *",
		"%eval in Demo::Vehicle : nonexistent + 1",
		"%eval in Demo::Vehicle::mass : 1 + 1",
	} {
		out, quit, err := s.RunMeta(line)
		if err != nil || quit {
			t.Fatalf("%s: err = %v, quit = %v", line, err, quit)
		}
		if len(out) == 0 {
			t.Errorf("%s: reported nothing", line)
		}
	}
}

// %help documents the form, which is how a user finds it.
func TestHelpDocumentsPinnedEval(t *testing.T) {
	wants(t, joinLines(helpText()), "%eval", "in <name> :")
}

// A pinned evaluation is one run, so the step budget bounds it as it bounds an
// unpinned one: a feature value read inside it does not reset the counter.
func TestEvalInInstanceIsBoundedByTheStepBudget(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	budgets := runtime.DefaultBudgets()
	budgets.MaxSteps = 6
	if err := s.SetBudgets(budgets); err != nil {
		t.Fatalf("setting budgets: %v", err)
	}
	run(t, s, "%instantiate Demo::Vehicle")

	// Nested to the right, so the feature value read is reached on the second step, long
	// before the budget runs out.
	expr := "mass"
	for i := 0; i < 60; i++ {
		expr = "mass + (" + expr + ")"
	}
	wants(t, run(t, s, "%eval in Demo::Vehicle : "+expr), "step limit exceeded")
	// The budget bounds one run, not the session.
	wants(t, run(t, s, "%eval in Demo::Vehicle : mass + 1.0"), "= 1501")
}
