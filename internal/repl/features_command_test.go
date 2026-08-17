package repl

import (
	"strings"
	"testing"
)

// deprecationLine is the note the old spelling adds ahead of its listing.
const deprecationLine = "note: %slots is deprecated — use %features"

// The rename is a spelling of one command: what %features lists is what %slots
// listed, down to the nested expansion and the object IDs.
func TestFeaturesMatchesSlotsListing(t *testing.T) {
	for _, fixture := range []string{"testdata/nested_part.sysml", "testdata/collection_slots.sysml"} {
		name := "Nested::Car"
		if strings.Contains(fixture, "collection") {
			name = "Coll::Rig"
		}
		t.Run(name, func(t *testing.T) {
			s := loadFixture(t, fixture)
			run(t, s, "%instantiate "+name)

			features := run(t, s, "%features "+name)
			slots := run(t, s, "%slots "+name)
			wants(t, features, "Features:")
			rejects(t, features, deprecationLine)
			if got := strings.TrimPrefix(slots, deprecationLine+"\n"); got != features {
				t.Errorf("%%slots listing differs from %%features:\n%s\nwant:\n%s", got, features)
			}
		})
	}
}

// The old spelling still works, and says what to write instead.
func TestSlotsIsADeprecatedAliasOfFeatures(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Vehicle")

	got := run(t, s, "%slots Vehicle")
	if !strings.HasPrefix(got, deprecationLine) {
		t.Errorf("%%slots did not lead with the deprecation note:\n%s", got)
	}
	wants(t, got, "mass = 1500.00")

	// Named without an argument, both spellings report their own usage.
	wants(t, run(t, s, "%features"), "usage: %features <name>")
	wants(t, run(t, s, "%slots"), deprecationLine, "usage: %slots <name>")
}

// %instantiate points the reader at the current spelling.
func TestInstantiateSuggestsFeatures(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instantiate Vehicle"), "Use %features Vehicle to inspect")
}

// The error paths are the command's, not the spelling's: an unresolved name, a
// definition with no object, and a slot that cannot be materialized read the
// same either way.
func TestFeaturesErrorPathsMatchSlots(t *testing.T) {
	t.Run("unresolved name", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features Nope"), "error: unresolved reference: Nope")
		wants(t, run(t, s, "%slots Nope"), deprecationLine, "error: unresolved reference: Nope")
	})

	t.Run("no instance", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features Vehicle"), "no instance of", "%instantiate")
		wants(t, run(t, s, "%slots Vehicle"), deprecationLine, "no instance of", "%instantiate")
	})

	t.Run("materialization error reaches the session status", func(t *testing.T) {
		for _, cmd := range []string{"%features", "%slots"} {
			s := submitted(t, unmaterializableModel)
			run(t, s, "%instantiate Demo::R")
			wants(t, run(t, s, cmd+" Demo::R"), "bad: <error:", "multiplicity violation")
			if !s.HasErrors() {
				t.Errorf("%s did not carry the materialization failure into the session status", cmd)
			}
			if n := len(s.MaterializationFailures()); n != 1 {
				t.Errorf("%s reported %d materialization failures, want 1", cmd, n)
			}
		}
	})

	t.Run("lost objects are explained", func(t *testing.T) {
		for _, cmd := range []string{"%features", "%slots"} {
			s := loadFixture(t, "testdata/vehicle_package.sysml")
			run(t, s, "%instantiate Demo::Vehicle")
			run(t, s, "%clear")
			wants(t, run(t, s, cmd+" Demo::Vehicle"), "the session was reset")
		}
	})
}

// Help lists the command under its current spelling only, while completion
// still offers both, since both are dispatched.
func TestFeaturesInHelpAndCompletion(t *testing.T) {
	help := strings.Join(helpText(), "\n")
	wants(t, help, "%features <name>")
	rejects(t, help, "%slots")

	got := NewSession().Complete("%fea", len("%fea"))
	if len(got.Candidates) != 1 || got.Candidates[0] != "%features" {
		t.Errorf("completing %%fea offered %v, want %%features", got.Candidates)
	}
	if !strings.Contains(strings.Join(NewSession().Complete("%s", len("%s")).Candidates, " "), "%slots") {
		t.Error("the deprecated spelling is dispatched but not offered by completion")
	}
}
