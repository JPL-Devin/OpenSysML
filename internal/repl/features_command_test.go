package repl

import (
	"strings"
	"testing"
)

// %features lists what an object holds for each feature of its type, nested
// expansion and object IDs included.
func TestFeaturesListsFeatureValues(t *testing.T) {
	for _, fixture := range []string{"testdata/nested_part.sysml", "testdata/collection_slots.sysml"} {
		name := "Nested::Car"
		if strings.Contains(fixture, "collection") {
			name = "Coll::Rig"
		}
		t.Run(name, func(t *testing.T) {
			s := loadFixture(t, fixture)
			run(t, s, "%instantiate "+name)
			wants(t, run(t, s, "%features "+name), "Features:")
		})
	}
}

// %slots was the pre-0.1.0 spelling and is gone, so it reads as any other
// unknown command rather than listing.
func TestSlotsIsNoLongerACommand(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Vehicle")

	got := run(t, s, "%slots Vehicle")
	wants(t, got, `unknown command "%slots"`)
	rejects(t, got, "mass = 1500.0")
	rejects(t, strings.Join(metaCommands(), " "), "%slots")
}

// %instantiate points the reader at the listing command.
func TestInstantiateSuggestsFeatures(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instantiate Vehicle"), "Use %features Vehicle to inspect")
}

// An unresolved name, a definition with no object, a feature value that cannot
// be materialized and a reset session each report rather than list.
func TestFeaturesErrorPaths(t *testing.T) {
	t.Run("no argument", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features"), "usage: %features <object>")
	})

	t.Run("unresolved name", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features Nope"), "error: unresolved reference: Nope")
	})

	t.Run("no instance", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		wants(t, run(t, s, "%features Vehicle"), "no instance of", "%instantiate")
	})

	t.Run("materialization error reaches the session status", func(t *testing.T) {
		s := submitted(t, unmaterializableModel)
		run(t, s, "%instantiate Demo::R")
		wants(t, run(t, s, "%features Demo::R"), "bad: <error:", "multiplicity violation")
		if !s.HasErrors() {
			t.Error("the listing did not carry the materialization failure into the session status")
		}
		if n := len(s.MaterializationFailures()); n != 1 {
			t.Errorf("%%features reported %d materialization failures, want 1", n)
		}
	})

	t.Run("lost objects are explained", func(t *testing.T) {
		s := loadFixture(t, "testdata/vehicle_package.sysml")
		run(t, s, "%instantiate Demo::Vehicle")
		run(t, s, "%clear")
		wants(t, run(t, s, "%features Demo::Vehicle"), "the session was reset")
	})
}

// Help and completion name the one spelling there is.
func TestFeaturesInHelpAndCompletion(t *testing.T) {
	help := strings.Join(helpText(), "\n")
	wants(t, help, "%features <object> [all|depth <n>] [json]")
	rejects(t, help, "%slots")

	got := NewSession().Complete("%fea", len("%fea"))
	if len(got.Candidates) != 1 || got.Candidates[0] != "%features" {
		t.Errorf("completing %%fea offered %v, want %%features", got.Candidates)
	}
	if strings.Contains(strings.Join(NewSession().Complete("%s", len("%s")).Candidates, " "), "%slots") {
		t.Error("completion still offers the removed spelling")
	}
}
