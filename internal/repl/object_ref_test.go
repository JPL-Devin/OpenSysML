package repl

import (
	"fmt"
	"strings"
	"testing"
)

// objectRefModel has a nested part and a multi-valued part feature, so an object
// reference can walk a single object, pick one of several, and go one level
// further down.
const objectRefModel = `package Garage {
	part def Hub {
		attribute bolts = 5;
	}
	part def Wheel {
		attribute radius = 0.3;
		part hub : Hub;
	}
	part def Car {
		attribute mass = 1500.0;
		part fl : Wheel;
		part wheels : Wheel[2];
	}
	part car : Car;
	part def Spare {
		attribute count = 1;
	}
}`

func garage(t *testing.T) *Session {
	t.Helper()
	return submitted(t, objectRefModel)
}

// %instantiate prints the id the runtime gives the object, and #<id> is how a
// command reaches it afterwards.
func TestObjectAddressedByID(t *testing.T) {
	s := garage(t)
	out := run(t, s, "%instantiate car")
	inst, ok := s.instances["Garage::car"]
	if !ok {
		t.Fatal("car was not recorded")
	}
	wants(t, out, fmt.Sprintf("ID: %d", inst.ID))

	id := fmt.Sprintf("#%d", inst.ID)
	wants(t, run(t, s, "%features "+id), fmt.Sprintf("Instance: %s (ID: %d)", id, inst.ID), "mass = 1500.0")
	wants(t, run(t, s, "%eval in "+id+" : mass * 2.0"), "3000.0", fmt.Sprintf("(on %s ID: %d)", id, inst.ID))
}

// A second %instantiate of a name re-points the name; the first object keeps its
// id and stays reachable by it, and %instances still lists it.
func TestInstantiateTwiceKeepsFirstObjectByID(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	first := s.instances["Garage::car"]
	again := run(t, s, "%instantiate car")
	second := s.instances["Garage::car"]
	if first.ID == second.ID {
		t.Fatalf("both instantiations report id %d", first.ID)
	}
	wants(t, again, fmt.Sprintf("object #%d is no longer named", first.ID), fmt.Sprintf("(address it as #%d)", first.ID))

	firstID := fmt.Sprintf("#%d", first.ID)
	wants(t, run(t, s, "%features "+firstID), fmt.Sprintf("Instance: %s (ID: %d)", firstID, first.ID))
	wants(t, run(t, s, "%features car"), fmt.Sprintf("(ID: %d)", second.ID))
	wants(t, run(t, s, "%instances"),
		fmt.Sprintf("Garage::car (ID: %d)", second.ID),
		fmt.Sprintf("%s (ID: %d, formerly Garage::car)", firstID, first.ID))

	// Each object has its own nested parts, reached through its own root.
	fl := run(t, s, "%features "+firstID+".fl")
	wants(t, fl, "Instance: "+firstID+"::fl")
	rejects(t, fl, "error")
}

// A reset counts the displaced object among what it drops, as %instances listed it.
func TestResetCountsDisplacedObjects(t *testing.T) {
	t.Run("clear", func(t *testing.T) {
		s := garage(t)
		run(t, s, "%instantiate car")
		run(t, s, "%instantiate car")
		wants(t, run(t, s, "%clear"), "2 instances were dropped because the session was reset")
		wants(t, run(t, s, "%instances"), "2 instances were dropped when the session was reset")
	})
	t.Run("budgets", func(t *testing.T) {
		s := garage(t)
		run(t, s, "%instantiate car")
		run(t, s, "%instantiate car")
		budgets := s.Budgets()
		budgets.MaxSteps++
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
		wants(t, run(t, s, "%instances"), "2 instances were dropped when the run bounds were changed")
	})
}

// Dotted paths rooted at a name or an id, and the `::` form, walk to the same
// nested object, which is reported under its declared root.
func TestDottedPathsReachNestedObjects(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]
	id := fmt.Sprintf("#%d", car.ID)

	byName := run(t, s, "%features car.fl")
	wants(t, byName, "Instance: Garage::car::fl (ID:", "radius = 0.3")
	wants(t, run(t, s, "%features Garage::car.fl"), "Instance: Garage::car::fl (ID:")
	wants(t, run(t, s, "%features Garage::car::fl"), "Instance: Garage::car::fl (ID:")
	wants(t, run(t, s, "%features car.fl.hub"), "Instance: Garage::car::fl::hub (ID:", "bolts = 5")
	wants(t, run(t, s, "%features "+id+".fl.hub"), "Instance: "+id+"::fl::hub (ID:", "bolts = 5")
	wants(t, run(t, s, "%features "+id+"::fl"), "Instance: "+id+"::fl (ID:")

	// The nested object's own id reaches it too, and reads the same values.
	flLine := strings.SplitN(byName, "\n", 2)[0]
	var flID int64
	if _, err := fmt.Sscanf(flLine, "Instance: Garage::car::fl (ID: %d)", &flID); err != nil {
		t.Fatalf("no id in %q: %v", flLine, err)
	}
	wants(t, run(t, s, fmt.Sprintf("%%features #%d", flID)), "radius = 0.3")
	wants(t, run(t, s, fmt.Sprintf("%%eval in #%d : radius", flID)), "0.3")
	wants(t, run(t, s, "%eval in car.fl : radius * 2.0"), "0.6", "(on Garage::car::fl ID:")
	wants(t, run(t, s, "%eval in "+id+".fl.hub : bolts"), "5", "(on "+id+"::fl::hub ID:")
}

// A multi-valued feature is walked by a 1-based index; leaving the index out, or
// giving one that names no element, says how many there are.
func TestMultiValuedFeatureNeedsIndex(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")

	wants(t, run(t, s, "%features car.wheels"), "error:", "wheels of Garage::car holds 2 objects: pick one by index, wheels[1] to wheels[2]")
	wants(t, run(t, s, "%features car.wheels[1]"), "Instance: Garage::car::wheels[1] (ID:", "radius = 0.3")
	wants(t, run(t, s, "%features car.wheels[2].hub"), "Instance: Garage::car::wheels[2]::hub (ID:", "bolts = 5")
	wants(t, run(t, s, "%features Garage::car::wheels[2]"), "Instance: Garage::car::wheels[2] (ID:")
	wants(t, run(t, s, "%features car.wheels[3]"), "error:", "wheels of Garage::car holds 2 objects, so wheels[3] names none (indexes run from 1 to 2)")
	wants(t, run(t, s, "%features car.wheels[0]"), "error:", "wheels[0] is not an index: elements are counted from 1")
	wants(t, run(t, s, "%features car.fl[1]"), "error:", "fl of Garage::car holds one value and takes no index: write fl, not fl[1]")
	wants(t, run(t, s, "%features car[1]"), "error:", "car[1] takes no index")

	one := run(t, s, "%features car.wheels[1]")
	two := run(t, s, "%features car.wheels[2]")
	if strings.SplitN(one, "\n", 2)[0] == strings.SplitN(two, "\n", 2)[0] {
		t.Errorf("wheels[1] and wheels[2] report the same object:\n%s", one)
	}
}

// Every command reports a bad reference in the same words: an unknown id lists
// the ids there are, a bad segment names the object and the segment, and a
// name with no object says so.
func TestObjectReferenceErrors(t *testing.T) {
	s := garage(t)
	wants(t, run(t, s, "%features #7"), "error: no object has id #7 (no objects have been created)")
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]

	// Nested parts are materialized when first read, so they join the listing.
	wants(t, run(t, s, "%features #999"), fmt.Sprintf("error: no object has id #999 (the objects are #%d)", car.ID))
	run(t, s, "%features car.fl")
	wants(t, run(t, s, "%features #999"), fmt.Sprintf("error: no object has id #999 (the objects are #%d, #", car.ID))
	wants(t, run(t, s, "%features car.nope"), "error: Garage::car has no feature \"nope\" (its features are fl, mass, wheels)")
	wants(t, run(t, s, "%features car.fl.hub.bolts"), "error: bolts of Garage::car::fl::hub holds a value (5), not an object")
	wants(t, run(t, s, "%features car.mass"), "error: mass of Garage::car holds a value (1500.0), not an object")
	wants(t, run(t, s, "%features #"), "error:", "an object id is written #<id>")
	wants(t, run(t, s, "%features #0"), "error:", "#0 is not an object id (ids count up from 1)")
	wants(t, run(t, s, "%features car."), "error:", `it ends in "." with no feature after it`)
	wants(t, run(t, s, "%features Spare"), "error: no instance of \"Garage::Spare\" (use %instantiate first)")
	wants(t, run(t, s, "%features Spare.count"), "error: no instance of \"Garage::Spare\" (use %instantiate first)")
	wants(t, run(t, s, "%features Nope.x"), "error: unresolved reference: Nope")

	// The other object-taking commands go through the same resolution.
	for _, cmd := range []string{"%eval in #999 : mass", "%invoke #999 go", "%state #999"} {
		out, _, err := s.RunMeta(cmd)
		msg := strings.Join(out, "\n")
		if err != nil {
			msg = err.Error()
		}
		wants(t, msg, "no object has id #999 (the objects are")
	}
	for _, cmd := range []string{"%eval in car.nope : mass", "%invoke car.nope go"} {
		out, _, err := s.RunMeta(cmd)
		msg := strings.Join(out, "\n")
		if err != nil {
			msg = err.Error()
		}
		wants(t, msg, `Garage::car has no feature "nope"`)
	}
}

// The debuggers take an object by id or path too: the machine an object
// exhibits, the operation it owns, and a behavior it performs.
func TestDebuggersAddressObjectsByID(t *testing.T) {
	t.Run("exhibited machine and operation", func(t *testing.T) {
		s := loadFixture(t, "testdata/exhibited_machine.sysml")
		first := objectIDIn(t, run(t, s, "%instantiate Obj::Monitor"))
		run(t, s, "%instantiate Obj::Monitor")

		wants(t, run(t, s, "%state #"+first), "exhibited by object #"+first, "Current state: idle")
		wants(t, run(t, s, "%current"), "idle")
		wants(t, run(t, s, "%invoke #"+first+" bumpBy n=4"), "✓ Invoked bumpBy on object #"+first)
		wants(t, run(t, s, "%features #"+first), "count = 5")
		// The named object is untouched.
		wants(t, run(t, s, "%features Obj::Monitor"), "count = 1")
		wants(t, run(t, s, "%eval in #"+first+" : count"), "5")
	})

	t.Run("performing object", func(t *testing.T) {
		s := NewSession()
		s.Submit("part def Holder { attribute size = 1.0; }")
		res := s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		if len(res.Diagnostics) > 0 {
			t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
		}
		id := objectIDIn(t, run(t, s, "%instantiate Holder"))
		wants(t, run(t, s, "%action tally #"+id), "Started action executor")
		wants(t, run(t, s, "%action tally #99"), "error:", "no object has id #99 (the objects are #"+id+")")
		s.Submit("part def Holder { attribute size = 2.0; }")
		wants(t, run(t, s, "%step"), "no active action session", "the object #"+id+" performing it was dropped")
	})
}

// Ids survive the carry-over an unrelated declaration triggers: the same
// objects, nested ones included, answer to the same ids and paths afterwards.
func TestObjectIDsStableAcrossCarryover(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]
	id := fmt.Sprintf("#%d", car.ID)
	before := run(t, s, "%features "+id+".fl")

	res := s.Submit("package Other { part def Unrelated; }")
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("declaration has errors: %v", errs)
	}
	if kept := s.instances["Garage::car"]; kept != car {
		t.Fatalf("car was replaced across the carry-over")
	}
	wants(t, run(t, s, "%features "+id), fmt.Sprintf("Instance: %s (ID: %d)", id, car.ID), "mass = 1500.0")
	after := run(t, s, "%features "+id+".fl")
	if strings.SplitN(before, "\n", 2)[0] != strings.SplitN(after, "\n", 2)[0] {
		t.Errorf("the nested object changed id across the carry-over:\n%s\n---\n%s", before, after)
	}
	wants(t, run(t, s, "%instances"), fmt.Sprintf("Garage::car (ID: %d)", car.ID))

	// An object a later %instantiate displaced is carried too, still by its id.
	run(t, s, "%instantiate car")
	second := s.instances["Garage::car"]
	res = s.Submit("package Other { part def Unrelated; part def Another; }")
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("declaration has errors: %v", errs)
	}
	if kept := s.instances["Garage::car"]; kept != second {
		t.Fatalf("the second car was replaced across the carry-over")
	}
	wants(t, run(t, s, "%features "+id), fmt.Sprintf("Instance: %s (ID: %d)", id, car.ID))
	wants(t, run(t, s, "%instances"),
		fmt.Sprintf("Garage::car (ID: %d)", second.ID),
		fmt.Sprintf("%s (ID: %d, formerly Garage::car)", id, car.ID))
}

// Where a command takes an object, completion offers the ids the session holds
// and walks a typed reference into the object-holding features of the object
// it names, an element of a multi-valued one picked by index; elsewhere names
// complete as before.
func TestCompleteObjectReferences(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	id := fmt.Sprintf("#%d", s.instances["Garage::car"].ID)

	complete := func(line string) Completion {
		return s.Complete(line, len(line))
	}
	tests := []struct {
		line    string
		prefix  string
		wants   []string
		rejects []string
	}{
		{line: "%features ", prefix: "", wants: []string{id, "car", "Garage"}},
		{line: "%features #", prefix: "#", wants: []string{id}, rejects: []string{"car"}},
		{line: "%features ca", prefix: "ca", wants: []string{"car"}, rejects: []string{id}},
		{line: "%features car.", prefix: "car.", wants: []string{"car.fl", "car.wheels[1]", "car.wheels[2]"}, rejects: []string{"car.mass"}},
		{line: "%features car.f", prefix: "car.f", wants: []string{"car.fl"}, rejects: []string{"car.wheels[1]"}},
		{line: "%features car.wheels[", prefix: "car.wheels[", wants: []string{"car.wheels[1]", "car.wheels[2]"}, rejects: []string{"car.fl"}},
		{line: "%features car.fl.", prefix: "car.fl.", wants: []string{"car.fl.hub"}},
		{line: "%features car::", prefix: "car::", wants: []string{"car::fl"}},
		{line: "%features " + id + ".", prefix: id + ".", wants: []string{id + ".fl", id + ".wheels[1]"}},
		{line: "%features Garage::", prefix: "Garage::", wants: []string{"Garage::car"}},
		{line: "%eval in ", prefix: "", wants: []string{id, "car"}},
		{line: "%eval in car.", prefix: "car.", wants: []string{"car.fl"}},
		{line: "%invoke ", prefix: "", wants: []string{id}},
		{line: "%state Sm ", prefix: "", wants: []string{id}},
		{line: "%action Act ", prefix: "", wants: []string{id}},
		{line: "%action ", prefix: "", rejects: []string{id}},
		{line: "%show ", prefix: "", wants: []string{"car"}, rejects: []string{id}},
		{line: "%eval in car : ", prefix: "", rejects: []string{id}},
		{line: "%features nobody.", prefix: "nobody.", rejects: []string{"car.fl"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := complete(tt.line)
			if got.Prefix != tt.prefix {
				t.Errorf("prefix = %q, want %q", got.Prefix, tt.prefix)
			}
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("want %q in %v", want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("did not want %q in %v", bad, got.Candidates)
				}
			}
		})
	}
}

// A quoted segment still being typed keeps the root and separator before it,
// after `.` and `::` alike, and an escaped quote inside it does not end it.
func TestCompleteQuotedSegments(t *testing.T) {
	s := submitted(t, `package Q {
	part def Gauge;
	part def Rack {
		part 'main gauge' : Gauge;
		part 'main valve' : Gauge;
		part 'rack\'s spare' : Gauge;
	}
	part 'the rack' : Rack;
}`)
	run(t, s, "%instantiate 'the rack'")
	before := s.rtCtx.InstanceIDs()

	tests := []struct {
		line    string
		prefix  string
		wants   []string
		rejects []string
	}{
		{line: "%features 'the ra", prefix: "'the ra", wants: []string{"'the rack'"}},
		{line: "%features 'the rack'.", prefix: "'the rack'.", wants: []string{"'the rack'.'main gauge'", "'the rack'.'main valve'", `'the rack'.'rack\'s spare'`}},
		{line: "%features 'the rack'.'main", prefix: "'the rack'.'main", wants: []string{"'the rack'.'main gauge'", "'the rack'.'main valve'"}, rejects: []string{`'the rack'.'rack\'s spare'`}},
		{line: "%features 'the rack'::'main g", prefix: "'the rack'::'main g", wants: []string{"'the rack'::'main gauge'"}, rejects: []string{"'the rack'::'main valve'"}},
		{line: `%features 'the rack'.'rack\'s`, prefix: `'the rack'.'rack\'s`, wants: []string{`'the rack'.'rack\'s spare'`}},
		{line: "%features Q::'the rack'.'main v", prefix: "Q::'the rack'.'main v", wants: []string{"Q::'the rack'.'main valve'"}},
		{line: "%eval in 'the rack'.'main", prefix: "'the rack'.'main", wants: []string{"'the rack'.'main gauge'"}},
		{line: "%state Sm 'the rack'.'main", prefix: "'the rack'.'main", wants: []string{"'the rack'.'main gauge'"}},
		{line: "%features 'the rack'.'nothing", prefix: "'the rack'.'nothing", rejects: []string{"'the rack'.'main gauge'"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := s.Complete(tt.line, len(tt.line))
			if got.Prefix != tt.prefix {
				t.Errorf("prefix = %q, want %q", got.Prefix, tt.prefix)
			}
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("want %q in %v", want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("did not want %q in %v", bad, got.Candidates)
				}
			}
		})
	}
	if after := s.rtCtx.InstanceIDs(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("completion materialized objects: ids %v, were %v", after, before)
	}
	if out := run(t, s, "%features 'the rack'.'main gauge'"); strings.Contains(out, "error") {
		t.Errorf("completed reference does not resolve:\n%s", out)
	}
}

// A multi-valued part is completed to the elements it holds, not to what its
// multiplicity admits: a ranged part materializes its lower bound, an optional
// one nothing — and every index offered is one a reference then resolves.
func TestCompleteIndexesOnlyHeldElements(t *testing.T) {
	s := submitted(t, `package Fleet {
	part def Wheel;
	part def Truck {
		part axles : Wheel[2..4];
		part spare : Wheel[0..1];
		part crew : Wheel[0..*];
	}
	part truck : Truck;
}`)
	run(t, s, "%instantiate truck")
	got := s.Complete("%features truck.", len("%features truck."))
	for _, want := range []string{"truck.axles[1]", "truck.axles[2]"} {
		if !contains(got.Candidates, want) {
			t.Errorf("want %q in %v", want, got.Candidates)
		}
	}
	for _, bad := range []string{"truck.axles[3]", "truck.axles[4]", "truck.spare[1]", "truck.crew[1]"} {
		if contains(got.Candidates, bad) {
			t.Errorf("did not want %q in %v", bad, got.Candidates)
		}
	}
	for _, c := range got.Candidates {
		if _, _, err := s.resolveObject(c); err != nil {
			t.Errorf("completion %q does not resolve: %v", c, err)
		}
	}
}

// A debugger performed by a nested object under quoted names keeps running over
// an unrelated declaration: the label the session holds the performer under is
// read back as the reference it spells.
func TestQuotedNestedPerformerSurvivesUnrelatedDeclaration(t *testing.T) {
	const model = `package 'Two Words' {
	part def Gauge {
		attribute count = 0;
	}
	part def Rack {
		part 'main gauge' : Gauge;
	}
	state def Check {
		entry; then checking;
		state checking {
			accept after 5 then checked;
		}
		state checked;
	}
	part 'the rack' : Rack;
}`
	t.Run("state", func(t *testing.T) {
		s := submitted(t, model)
		run(t, s, "%instantiate 'the rack'")
		wants(t, run(t, s, "%state 'Two Words'::Check 'the rack'.'main gauge'"), "Started state machine executor")
		if res := s.Submit("package Other { part def Unrelated; }"); len(res.Notices) > 0 {
			t.Fatalf("unrelated declaration reported %v", res.Notices)
		}
		wants(t, run(t, s, "%current"), "checking")
		wants(t, run(t, s, "%advance 5"), "Current state: checked")
	})
	t.Run("action", func(t *testing.T) {
		s := submitted(t, model)
		res := s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
		if len(res.Diagnostics) > 0 {
			t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
		}
		run(t, s, "%instantiate 'the rack'")
		wants(t, run(t, s, "%action tally 'the rack'.'main gauge'"), "Started action executor")
		if res := s.Submit("package Other { part def Unrelated; }"); len(res.Notices) > 0 {
			t.Fatalf("unrelated declaration reported %v", res.Notices)
		}
		rejects(t, run(t, s, "%step"), "no active action session")
	})
}

// Completion reads and never materializes: it follows a part not yet reached by
// a command by type, and pressing Tab leaves the runtime as it found it.
func TestCompletionMaterializesNothing(t *testing.T) {
	s := garage(t)
	run(t, s, "%instantiate car")
	car := s.instances["Garage::car"]
	before := s.rtCtx.InstanceIDs()

	for _, line := range []string{"%features car.", "%features car.fl.", "%features car.wheels[2].", "%eval in car.wheels[1].", "%features #1.fl.hu"} {
		got := s.Complete(line, len(line))
		if len(got.Candidates) == 0 {
			t.Errorf("%q offered nothing", line)
		}
	}
	if after := s.rtCtx.InstanceIDs(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("completion changed the objects held: %v, was %v", after, before)
	}
	for _, name := range []string{"fl", "wheels"} {
		if car.FeatureValues[name].Materialized {
			t.Errorf("completion materialized car.%s", name)
		}
	}
	wants(t, run(t, s, "%features car.wheels[2].hub"), "bolts = 5")
	rejects(t, run(t, s, "%features car.wheels[3]"), "bolts")
}

// A materialized state machine exhibits none, so a reference to it — by id or by
// path — debugs a run of its machine, as its declared name always has.
func TestStateMachineObjectsByReference(t *testing.T) {
	s := submitted(t, `package Plant {
	state def Modes {
		entry; then off;
		state off {
			accept after 2 then on;
		}
		state on;
	}
	part def Monitor {
		state modes : Modes;
	}
	part monitor : Monitor;
}`)
	id := objectIDIn(t, run(t, s, "%instantiate Plant::Modes"))
	wants(t, run(t, s, "%state Plant::Modes"), "Started state machine executor", "Current state: off")
	wants(t, run(t, s, "%state #"+id), "Started state machine executor", "Current state: off")
	wants(t, run(t, s, "%advance 2"), "Current state: on")
	run(t, s, "%instantiate monitor")
	wants(t, run(t, s, "%state monitor.modes"), "Started state machine executor", "Current state: off")
	wants(t, run(t, s, "%advance 2"), "Current state: on")
	wants(t, run(t, s, "%state monitor.nope"), "error:", `Plant::monitor has no feature "nope"`)
}

// An object displaced from its name still carries its type's conditions, so an
// unpinned check or evaluation names it as one of the objects it could be about
// instead of quietly answering about the currently named one.
func TestDisplacedObjectsCarryImplicitSubjects(t *testing.T) {
	const lab = `package Lab {
	part def Sensor {
		attribute reading : ScalarValues::Real = 1.0;
		constraint inRange { reading < 5.0 }
	}
	part def Rack { part gauge : Sensor; }
	part sensor : Sensor;
	part rack : Rack;
}`
	t.Run("displaced object", func(t *testing.T) {
		s := submitted(t, lab)
		first := objectIDIn(t, run(t, s, "%instantiate sensor"))
		wants(t, run(t, s, "%constraint Lab::Sensor::inRange"), "passed (on Lab::sensor ID: "+first+")")
		second := objectIDIn(t, run(t, s, "%instantiate sensor"))
		if first == second {
			t.Fatalf("a second %%instantiate reused id #%s", first)
		}
		both := "carried by more than one object of this session (Lab::sensor, #" + first + ")"
		wants(t, run(t, s, "%constraint Lab::Sensor::inRange"), "error:", both)
		wants(t, run(t, s, "%eval Lab::Sensor::reading"), "error:", both)
		// Naming the object settles the question.
		wants(t, run(t, s, "%eval in #"+first+" : reading"), "1.0")
	})

	t.Run("part of a displaced object", func(t *testing.T) {
		s := submitted(t, lab)
		rack := objectIDIn(t, run(t, s, "%instantiate rack"))
		run(t, s, "%instantiate rack")
		wants(t, run(t, s, "%eval Lab::Sensor::reading"),
			"carried by more than one object of this session (Lab::rack::gauge, #"+rack+"::gauge)")
	})
}
