package repl

import "testing"

// TestSatisfyVerdicts checks the entry point an anonymous satisfaction assertion
// is reached through: the element that states it, or the whole session.
func TestSatisfyVerdicts(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_landing.sysml")

	all := run(t, s, "%satisfy")
	wants(t, all,
		"✓ satisfy touchdown by slowLander holds",
		"✗ satisfy touchdown by fastLander fails",
		"Required condition evaluated to false: lander.verticalSpeed <= maxVerticalSpeed",
		"✓ not satisfy touchdown by fastLander holds",
	)

	wants(t, run(t, s, "%satisfy Landing::analysisContext"), "✓ satisfy touchdown by slowLander holds")
	wants(t, run(t, s, "%satisfy nosuch"), `symbol "nosuch" not found`)
	// A requirement is not itself an assertion, and states none.
	wants(t, run(t, s, "%satisfy touchdown"), "no satisfaction assertion in Landing::touchdown")

	empty := NewSession()
	wants(t, run(t, empty, "%satisfy"), "no declarations loaded")
}

// TestSatisfyUsesInstantiatedSubject checks that a verdict is about the object
// the session created for the subject, not a fresh one.
func TestSatisfyUsesInstantiatedSubject(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_landing.sysml")
	wants(t, run(t, s, "%instantiate Landing::slowLander"), "✓ Created instance of Landing::slowLander")
	wants(t, run(t, s, "%satisfy Landing::analysisContext"),
		"✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)")
}

// TestRepeatedSatisfyKeepsItsSubject checks that a subject created by %satisfy
// itself is kept, so repeating the command is about the same object.
func TestRepeatedSatisfyKeepsItsSubject(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_landing.sysml")
	first := "✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)"
	wants(t, run(t, s, "%satisfy Landing::analysisContext"), first)
	wants(t, run(t, s, "%satisfy Landing::analysisContext"), first)
	wants(t, run(t, s, "%slots Landing::slowLander"), "verticalSpeed")
}
