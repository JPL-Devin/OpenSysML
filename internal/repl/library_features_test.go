package repl

import "testing"

// %features lists the Systems and Domain library features an object inherits
// alongside its own, and %eval reads them; the Kernel frame stays out.
func TestFeaturesListsInheritedLibraryFeatures(t *testing.T) {
	s := loadFixture(t, "testdata/library_box.sysml")
	run(t, s, "%instantiate Demo::box")

	// The object's own lines are indented two spaces; the faces it expands
	// nest deeper and must not crowd them out of the listing.
	got := run(t, s, "%features Demo::box")
	wants(t, got, "\n  length = 2 [m]", "\n  width = 1 [m]", "\n  height = 1 [m]",
		"\n  isSolid = true", "\n  voids = []", "\n  shape = <unset>")
	rejects(t, got, "self =", "portions =", "timeSlices =", "snapshots =", "startShot =")

	wants(t, run(t, s, "%eval box.isSolid"), "true")
	wants(t, run(t, s, "%eval box.voids"), "[]")
}

// A requirement reads the subject, actors, stakeholders and constraint
// collections Requirements::RequirementCheck gives it.
func TestFeaturesListsInheritedRequirementFeatures(t *testing.T) {
	s := loadFixture(t, "testdata/library_box.sysml")
	run(t, s, "%instantiate Demo::massLimit")

	got := run(t, s, "%features Demo::massLimit")
	wants(t, got, "\n  limit = 10 [kg]", "\n  actors = []", "\n  stakeholders = []",
		"\n  assumptions = []", "\n  constraints = []", "\n  subj = ")
	rejects(t, got, "self =", "portions =", "timeSlices =")
}
