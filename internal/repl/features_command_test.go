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
		wants(t, run(t, s, "%features"), "usage: %features <name>")
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
	wants(t, help, "%features <name>")
	rejects(t, help, "%slots")

	got := NewSession().Complete("%fea", len("%fea"))
	if len(got.Candidates) != 1 || got.Candidates[0] != "%features" {
		t.Errorf("completing %%fea offered %v, want %%features", got.Candidates)
	}
	if strings.Contains(strings.Join(NewSession().Complete("%s", len("%s")).Candidates, " "), "%slots") {
		t.Error("completion still offers the removed spelling")
	}
}

// behaviorsModel is a part whose type exhibits a machine, performs an action,
// and declares an action and a state it neither performs nor exhibits.
const behaviorsModel = `private import ScalarValues::*;
state def LampModes {
    entry; then off;
    state off { accept after 10 then on; }
    state on;
}
part def Lamp {
    attribute watts : Real = 60.0;
    exhibit state modes : LampModes;
    action blink { in n : Real; }
    perform action tick { first bump; action bump { assign watts := watts + 2.0; } }
    state standalone;
}
part lamp : Lamp;
part def Rig { attribute rpm : Integer = 0; part inner : Lamp; }
part rig : Rig;`

// A state or action a type declares holds no value, so %features lists it under
// its own heading with what the object is doing with it: the active state of the
// machine the object exhibits — the state %current reports, before and after the
// debugger drives it — the execution state of the action it performs, and "not
// running" for a behavior the object does not run.
func TestFeaturesListsBehaviorsUnderTheirOwnHeading(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(behaviorsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate lamp")

	got := run(t, s, "%features lamp")
	wants(t, got,
		"Features:\n  watts = 62.0\nBehaviors:",
		"  modes: exhibited state machine, current state off",
		"  tick: performed action, completed",
		"  blink: action, not running",
		"  standalone: state, not running",
	)
	rejects(t, got, "<unknown>", "off = ", "on = ", "bump = ", "n = ", "modes = Instance")

	wants(t, run(t, s, "%state lamp"), "Current state: off")
	wants(t, run(t, s, "%current"), "Current state: off")
	wants(t, run(t, s, "%features lamp"), "modes: exhibited state machine, current state off")

	wants(t, run(t, s, "%advance 10"), "Current state: on")
	wants(t, run(t, s, "%current"), "Current state: on")
	got = run(t, s, "%features lamp")
	wants(t, got, "modes: exhibited state machine, current state on")
	rejects(t, got, "current state off", "<unknown>")
}

// A nested object's behaviors are listed under its own row, not mixed into the
// listed object's.
func TestFeaturesListsNestedBehaviorsUnderTheNestedObject(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(behaviorsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate rig")

	got := run(t, s, "%features rig")
	wants(t, got,
		"  inner = Instance(ID: ",
		"    watts = 62.0\n    Behaviors:\n      modes: exhibited state machine, current state off",
		"      tick: performed action, completed",
	)
	rejects(t, got, "\nBehaviors:", "<unknown>")
}

// An object of a state definition has states, not values: they are listed as
// behaviors the object does not run rather than as <unknown> values.
func TestFeaturesOfAMachineObjectListsItsStates(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(behaviorsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate LampModes")

	got := run(t, s, "%features LampModes")
	wants(t, got, "Behaviors:", "  off: state, not running", "  on: state, not running")
	rejects(t, got, "<unknown>")
}
