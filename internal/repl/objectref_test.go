package repl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// %state <machine> <object>, for the machine the object exhibits, attaches to the
// running machine and says so: its entry action does not run against the object's
// slots a second time.
func TestStateOverTheExhibitedMachineAttachesToIt(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::rover")
	wants(t, run(t, s, "%features Fleet::rover"), "level = 10", `log = "W"`)

	got := run(t, s, "%state Fleet::Rover::modes Fleet::rover")
	wants(t, got, `Debugging state machine "modes" exhibited by object #`, "note:",
		`already exhibits "Fleet::Rover::modes"`, "attaches to that running machine", "`%state Fleet::rover`")
	if strings.Contains(got, "Started state machine executor") {
		t.Errorf("a second performance of the exhibited machine was started:\n%s", got)
	}

	features := run(t, s, "%features Fleet::rover")
	wants(t, features, "level = 10", `log = "W"`)
	for _, twice := range []string{"level = 20", `log = "WW"`} {
		if strings.Contains(features, twice) {
			t.Errorf("the entry action ran twice (%s):\n%s", twice, features)
		}
	}

	// The session drives the object's own machine.
	wants(t, run(t, s, "%advance 5"), "Current state: moving")
	wants(t, run(t, s, "%features Fleet::rover"), `log = "WM"`)
}

// A machine the object merely performs is still a detached run, as before.
func TestStateOverAPerformedMachineStillStartsIt(t *testing.T) {
	s := loadFixture(t, "testdata/performed_machine.sysml")
	run(t, s, "%instantiate Two::g")

	wants(t, run(t, s, "%state Two::Check Two::g"), "Started state machine executor")
}

// %state addresses a nested part by a feature path from a top-level object and by
// the identity the prompt prints, in both the one- and two-argument forms.
func TestStateAddressesANestedPartByPathAndById(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")

	byPath := run(t, s, "%state Fleet::driver.r")
	wants(t, byPath, `Debugging state machine "modes" exhibited by object #`, `of "Fleet::driver::r"`, "Current state: waiting")
	id := objectIDIn(t, byPath)

	wants(t, run(t, s, "%state #"+id), `exhibited by object #`+id+` of "Fleet::driver::r"`)
	wants(t, run(t, s, "%state Fleet::driver::r"), `exhibited by object #`+id+` of "Fleet::driver::r"`)

	attached := run(t, s, "%state Fleet::Rover::modes Fleet::driver.r")
	wants(t, attached, `exhibited by object #`+id, "note:", "attaches to that running machine", "`%state Fleet::driver.r`")
	wants(t, run(t, s, "%state Fleet::Rover::modes #"+id), `exhibited by object #`+id, "note:", "`%state #"+id+"`")

	wants(t, run(t, s, "%features Fleet::driver.r"), "ID: "+id, "level = 10", `log = "W"`)
	wants(t, run(t, s, "%features #"+id), "Fleet::driver::r", "level = 10")
}

// %invoke addresses a nested part by path and by identity, and what the operation
// writes is that object's slot.
func TestInvokeAddressesANestedPartByPathAndById(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")
	id := objectIDIn(t, run(t, s, "%features Fleet::driver.r"))

	wants(t, run(t, s, "%invoke Fleet::driver.r bump"), "✓ Invoked bump on object #"+id+` of "Fleet::driver::r"`)
	wants(t, run(t, s, "%invoke #"+id+" bump"), "✓ Invoked bump on object #"+id+` of "Fleet::driver::r"`)
	wants(t, run(t, s, "%features Fleet::driver.r"), "level = 12")
	wants(t, run(t, s, "%features Fleet::driver"), "level = 12")
}

// A qualified path through a usage (Fleet::driver::r) resolves to the feature's
// declaration (Fleet::Driver::r); with the definition instantiated too, the path
// still denotes the usage's part, on every command taking an object.
func TestQualifiedPathDenotesTheUsageTypedNotItsDefinition(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::Driver")
	run(t, s, "%instantiate Fleet::driver")
	defR := objectIDIn(t, run(t, s, "%features Fleet::Driver.r"))
	usageR := objectIDIn(t, run(t, s, "%features Fleet::driver.r"))
	if defR == usageR {
		t.Fatalf("the definition's and the usage's parts are one object #%s", defR)
	}

	wants(t, run(t, s, "%features Fleet::driver::r"), "ID: "+usageR, `Instance: Fleet::driver::r`)
	wants(t, run(t, s, "%features Fleet::Driver::r"), "ID: "+defR, `Instance: Fleet::Driver::r`)

	wants(t, run(t, s, "%invoke Fleet::driver::r bump"), "on object #"+usageR+` of "Fleet::driver::r"`)
	wants(t, run(t, s, "%features Fleet::driver"), "level = 11")
	wants(t, run(t, s, "%features Fleet::Driver"), "level = 10")

	wants(t, run(t, s, "%state Fleet::driver::r"), `exhibited by object #`+usageR+` of "Fleet::driver::r"`)
	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::driver::r"), `exhibited by object #`+usageR, "note:")
	wants(t, run(t, s, "%state Fleet::Driver::r"), `exhibited by object #`+defR+` of "Fleet::Driver::r"`)
}

// A member of a multi-valued part is reached by no path, since the owner's
// feature holds it among others: the prompt names it by identity, and a session
// attached to it by identity survives an unrelated declaration.
func TestCollectionMemberIsNamedByIdentity(t *testing.T) {
	s := loadFixture(t, "testdata/collection_machine.sysml")
	run(t, s, "%instantiate Depot::garage")
	listing := run(t, s, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	first := objectIDIn(t, bays)
	id := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])
	if first == id {
		t.Fatalf("bays holds one member #%s, want two", id)
	}

	features := run(t, s, "%features #"+id)
	wants(t, features, "Instance: #"+id+" (ID: "+id+")", "level = 10")
	if strings.Contains(features, "Depot::garage::bays") {
		t.Errorf("a member of bays was named as the whole collection:\n%s", features)
	}
	if _, _, err := s.objectRef("Depot::garage::bays"); err == nil {
		t.Error("the collection's path reached an object")
	}

	got := run(t, s, "%state #"+id)
	wants(t, got, `Debugging state machine "modes" exhibited by object #`+id, "Current state: waiting")
	if strings.Contains(got, "Depot::garage::bays") {
		t.Errorf("the member's machine was reported under the collection's name:\n%s", got)
	}
	if s.stateExec.selfFQN != "#"+id {
		t.Errorf("the session holds the member as %q, want #%s", s.stateExec.selfFQN, id)
	}

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	if hasNotice(res, "debugging session") {
		t.Errorf("the session over the member's machine was ended:\n%s", strings.Join(res.Notices, "\n"))
	}
	wants(t, run(t, s, "%current"), "waiting")
	wants(t, run(t, s, "%advance 5"), "Current state: moving")

	// The advance drove that member alone: the other is where it started.
	wants(t, run(t, s, "%state #"+first), `exhibited by object #`+first, "Current state: waiting")
	wants(t, run(t, s, "%state #"+id), `exhibited by object #`+id, "Current state: moving")
}

// A path that stops short of an object is a typed error naming the segment that
// reached none and why, on every command taking an object.
func TestObjectPathErrorsNameTheSegment(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")

	cases := []struct {
		arg, segment, reason string
	}{
		{"Fleet::driver.x", "x", `object #1 of "Fleet::driver" has no feature "x"`},
		{"Fleet::driver.r.level", "level", `feature "level" of object #`},
		{"Fleet::driver.r.level", "level", "holds 10, which is not an object"},
		{"Fleet::driver.r.nothing", "nothing", `has no feature "nothing"`},
	}
	for _, c := range cases {
		_, _, err := s.objectRef(c.arg)
		var perr *ObjectPathError
		if !errors.As(err, &perr) {
			t.Errorf("%s: got %v, want an ObjectPathError", c.arg, err)
			continue
		}
		if perr.Path != c.arg || perr.Segment != c.segment || !strings.Contains(perr.Reason, c.reason) {
			t.Errorf("%s: got %+v, want segment %q, reason %q", c.arg, perr, c.segment, c.reason)
		}
		want := c.arg + ` reaches no object at "` + c.segment + `"`
		wants(t, run(t, s, "%state "+c.arg), "error:", want, c.reason)
		wants(t, run(t, s, "%state Fleet::Rover::modes "+c.arg), "error:", want)
		wants(t, run(t, s, "%invoke "+c.arg+" bump"), "error:", want)
		wants(t, run(t, s, "%features "+c.arg), "error:", want)
	}

	_, _, err := s.objectRef("#99")
	var nerr *NoObjectError
	if !errors.As(err, &nerr) || nerr.ID != 99 {
		t.Errorf("#99: got %v, want a NoObjectError for 99", err)
	}
	wants(t, run(t, s, "%state #99"), "error:", "no object #99 in this session")
	wants(t, run(t, s, "%invoke #99 bump"), "error:", "no object #99 in this session")
}

// When only the definition a usage is typed by was materialized, the error says
// so and names the usage to instantiate; the other way round it names the usage
// whose object exists. With nothing related, the plain hint stands.
func TestNotInstantiatedNamesWhatToInstantiate(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")

	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::rover"), `error: no instance of "Fleet::rover" (use %instantiate first)`)

	defOnly := objectIDIn(t, run(t, s, "%instantiate Fleet::Rover"))
	got := run(t, s, "%state Fleet::Rover::modes Fleet::rover")
	wants(t, got, `error: no instance of the usage "Fleet::rover"`,
		`object #`+defOnly+` of "Fleet::Rover" is of its definition "Fleet::Rover", not of the usage`,
		"use %instantiate Fleet::rover to create the usage's object", "or name Fleet::Rover to address it")
	wants(t, run(t, s, "%invoke Fleet::rover bump"), `no instance of the usage "Fleet::rover"`, "%instantiate Fleet::rover")

	_, _, err := s.objectRef("Fleet::rover")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || !nerr.UsageAsked || nerr.Definition != "Fleet::Rover" || len(nerr.Objects) != 1 || nerr.Objects[0].Name != "Fleet::Rover" {
		t.Errorf("got %v (%+v), want a NotInstantiatedError naming Fleet::Rover's object", err, nerr)
	}

	// A nested object of the definition counts under the path reaching it. A part
	// whose type exhibits a machine is created, and its machine run, with the
	// object holding it, so the error names it as soon as that object exists.
	run(t, s, "%instantiate Fleet::driver")
	if fv := s.instances["Fleet::driver"].FeatureValues["r"]; !fv.Materialized {
		t.Errorf("instantiating Fleet::driver left its machine-exhibiting part unread: %+v", fv)
	}
	wants(t, run(t, s, "%invoke Fleet::rover bump"), `no instance of the usage "Fleet::rover"`)
	wants(t, run(t, s, "%features Fleet::driver.r"), `log = "W"`)
	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::rover"), `objects #`+defOnly+` of "Fleet::Rover"`,
		`of "Fleet::driver::r"`, "or name Fleet::Rover or Fleet::driver::r to address one of them")

	// Asking for the definition when only a usage's object exists names the usage.
	fresh := loadFixture(t, "testdata/nested_machine.sysml")
	usage := objectIDIn(t, run(t, fresh, "%instantiate Fleet::rover"))
	wants(t, run(t, fresh, "%invoke Fleet::Rover bump"), `error: no instance of the definition "Fleet::Rover" itself`,
		`object #`+usage+` of "Fleet::rover" is typed by it`, "name Fleet::rover to address it", "%instantiate Fleet::Rover")
}

// A usage that reaches its definition only through the usage it subsets is still
// of that definition, so objects of it are named the same way.
func TestNotInstantiatedFindsTheDefinitionThroughASubsettedUsage(t *testing.T) {
	s := loadFixture(t, "testdata/indirect_usage.sysml")
	wants(t, run(t, s, "%invoke Fleet::scout bump"), `error: no instance of "Fleet::scout" (use %instantiate first)`)

	defOnly := objectIDIn(t, run(t, s, "%instantiate Fleet::Rover"))
	rover := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	wants(t, run(t, s, "%invoke Fleet::scout bump"), `error: no instance of the usage "Fleet::scout"`,
		`objects #`+defOnly+` of "Fleet::Rover", #`+rover+` of "Fleet::rover" are of its definition "Fleet::Rover", not of the usage`,
		"use %instantiate Fleet::scout to create the usage's object", "or name Fleet::Rover or Fleet::rover to address one of them")
	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::scout"), `no instance of the usage "Fleet::scout"`, `of its definition "Fleet::Rover"`)

	_, _, err := s.objectRef("Fleet::scout")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || !nerr.UsageAsked || nerr.Definition != "Fleet::Rover" || len(nerr.Objects) != 2 {
		t.Errorf("got %v (%+v), want a NotInstantiatedError naming Fleet::Rover's two objects", err, nerr)
	}
}

// A session over a nested part's machine survives an unrelated declaration as one
// over a top-level object does: the object keeps its identity and the session
// follows the restarted machine.
func TestNestedObjectMachineSurvivesAnUnrelatedDeclaration(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")
	id := objectIDIn(t, run(t, s, "%state Fleet::driver.r"))
	wants(t, run(t, s, "%advance 5"), "Current state: moving")

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	if hasNotice(res, "debugging session") {
		t.Errorf("the session over the nested part's machine was ended:\n%s", strings.Join(res.Notices, "\n"))
	}

	wants(t, run(t, s, "%current"), "waiting")
	wants(t, run(t, s, "%features #"+id), "Fleet::driver::r", `log = "W"`)
	wants(t, run(t, s, "%advance 5"), "Current state: moving")
}

// A session attached to the second of two exhibited machines follows that
// machine through the restart an unrelated declaration causes, not the first.
func TestSecondExhibitedMachineSurvivesAnUnrelatedDeclaration(t *testing.T) {
	s := loadFixture(t, "testdata/two_machines.sysml")
	run(t, s, "%instantiate Pair::gauge")
	wants(t, run(t, s, "%state Pair::Gauge::link Pair::gauge"), `Debugging state machine "link"`, "Current state: idle")

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	if hasNotice(res, "debugging session") {
		t.Errorf("the session over the second machine was ended:\n%s", strings.Join(res.Notices, "\n"))
	}

	current := run(t, s, "%current")
	wants(t, current, "idle")
	if strings.Contains(current, "off") {
		t.Errorf("the session moved to the first exhibited machine:\n%s", current)
	}
	wants(t, run(t, s, "%advance 3"), "Current state: busy")
	wants(t, run(t, s, "%features Pair::gauge"), "power", "off", "link", "busy")
}

// A definition an object exhibits as the body of two usages names no one
// running machine, so %state over it refuses and names the usages that would,
// rather than attaching to whichever was declared first. Naming a usage attaches.
func TestStateOverASharedDefinitionNamesTheUsages(t *testing.T) {
	s := loadFixture(t, "testdata/shared_machine.sysml")
	run(t, s, "%instantiate Shared::lamp")

	_, err := s.startStateMachine("Shared::Blink", []string{"Shared::lamp"})
	var aerr *AmbiguousMachineError
	if !errors.As(err, &aerr) {
		t.Fatalf("got %v, want an AmbiguousMachineError", err)
	}
	if aerr.Machine != "Shared::Blink" || strings.Join(aerr.Usages, ",") != "Shared::Lamp::front,Shared::Lamp::rear" {
		t.Errorf("got %+v, want both exhibited usages of Shared::Blink", aerr)
	}
	if s.stateExec != nil {
		t.Errorf("a machine was attached to despite the ambiguity: %q", s.stateExec.machine)
	}

	got := run(t, s, "%state Shared::Blink Shared::lamp")
	wants(t, got, "error:", `object #1 of "Shared::lamp" exhibits "Shared::Blink" as 2 machines`,
		"name the exhibited usage instead", "Shared::Lamp::front or Shared::Lamp::rear")
	for _, attached := range []string{"Debugging state machine", "Started state machine executor"} {
		if strings.Contains(got, attached) {
			t.Errorf("a machine was selected despite the ambiguity:\n%s", got)
		}
	}

	wants(t, run(t, s, "%state Shared::Lamp::rear Shared::lamp"), `Debugging state machine "rear"`, "note:", "Current state: dark")
	wants(t, run(t, s, "%advance 2"), "Current state: lit")
	wants(t, run(t, s, "%features Shared::lamp"), "front", "dark", "rear", "lit")
}

// A segment whose feature value the runtime could not materialize is not a missing
// feature: the path error carries the runtime's reason and its typed cause, and
// the failure reaches the session status as any other command's would.
func TestObjectPathErrorKeepsTheMaterializationFailure(t *testing.T) {
	s := loadFixture(t, "testdata/shared_machine.sysml")
	run(t, s, "%instantiate Shared::lamp")

	_, _, err := s.objectRef("Shared::lamp.spare")
	var perr *ObjectPathError
	if !errors.As(err, &perr) {
		t.Fatalf("got %v, want an ObjectPathError", err)
	}
	if perr.Segment != "spare" || strings.Contains(perr.Reason, "has no feature") {
		t.Errorf("got %+v, want the segment spare reported as failing to materialize", perr)
	}
	wants(t, perr.Reason, `feature "spare" of object #1 could not be materialized`, "multiplicity violation")
	if !errors.Is(err, runtime.ErrFeatureValueMaterialization) || !errors.Is(err, runtime.ErrMultiplicityViolation) {
		t.Errorf("%v does not carry the runtime's materialization failure", err)
	}

	want := `Shared::lamp.spare reaches no object at "spare"`
	wants(t, run(t, s, "%state Shared::lamp.spare"), "error:", want, "could not be materialized", "multiplicity violation")
	wants(t, run(t, s, "%invoke Shared::lamp.spare flip"), "error:", want, "multiplicity violation")
	if got := s.MaterializationFailures(); len(got) != 2 || !errors.Is(got[0], runtime.ErrMultiplicityViolation) {
		t.Errorf("materialization failures = %v, want the multiplicity violation from each command", got)
	}

	if v := s.RunStateMachine("Shared::lamp.spare"); v.Status != VerdictUnresolved || !strings.Contains(strings.Join(v.Lines, "\n"), "multiplicity violation") {
		t.Errorf("non-interactive run: got %+v, want an unresolved verdict naming the multiplicity violation", v)
	}
}

// Members of a multi-valued part are among the objects a not-instantiated error
// names, by identity since no path reaches one, and a feature they all carry is a
// question between them. Members exhibiting a machine exist with their holder.
func TestNotInstantiatedNamesCollectionMembersByIdentity(t *testing.T) {
	s := loadFixture(t, "testdata/collection_machine.sysml")
	run(t, s, "%instantiate Depot::garage")
	if fv := s.instances["Depot::garage"].FeatureValues["bays"]; !fv.Materialized {
		t.Fatalf("instantiating Depot::garage left its machine-exhibiting members unread: %+v", fv)
	}

	listing := run(t, s, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	first := objectIDIn(t, bays)
	second := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])

	got := run(t, s, "%state Depot::Rover::modes Depot::Rover")
	wants(t, got, `error: no instance of the definition "Depot::Rover" itself`,
		"objects #"+first+", #"+second+" are typed by it", "name #"+first+" or #"+second+" to address one of them",
		"%instantiate Depot::Rover")
	if strings.Contains(got, "bays") || strings.Contains(got, "'#") {
		t.Errorf("a member is named by the collection's path, or its identity quoted:\n%s", got)
	}
	_, _, err := s.objectRef("Depot::Rover")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || nerr.UsageAsked || len(nerr.Objects) != 2 {
		t.Fatalf("got %v (%+v), want a NotInstantiatedError naming both members", err, nerr)
	}
	for _, o := range nerr.Objects {
		if o.Name != "" {
			t.Errorf("member #%d is named %q, want its identity alone", o.ID, o.Name)
		}
	}
	if s.stateExec != nil {
		t.Error("a machine was attached to despite the error")
	}

	wants(t, run(t, s, "%eval Depot::Rover::level"), "error:", "carried by more than one object of this session (#"+first+", #"+second+")")
}

// A member of a multi-valued part that a single-valued part of the same owner
// also holds is reached by that part's path, not its identity — whichever of the
// owner's feature values is read first — and named so by every surface.
func TestCollectionMemberHeldAloneElsewhereKeepsItsPath(t *testing.T) {
	s := loadFixture(t, "testdata/shared_member.sysml")
	run(t, s, "%instantiate Depot::garage")
	listing := run(t, s, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	front := objectIDIn(t, bays)
	other := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])
	wants(t, listing, "front = Instance(ID: "+front+")", "lead = Instance(ID: "+front+")")

	got := run(t, s, "%state Depot::Rover::modes Depot::Rover")
	wants(t, got, `objects #`+front+` of "Depot::garage::front", #`+other+` are typed by it`,
		"name Depot::garage::front or #"+other+" to address one of them")
	_, _, err := s.objectRef("Depot::Rover")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || len(nerr.Objects) != 2 || nerr.Objects[0].Name != "Depot::garage::front" || nerr.Objects[1].Name != "" {
		t.Errorf("got %v (%+v), want the shared member under front's path and the other by identity", err, nerr)
	}

	wants(t, run(t, s, "%eval Depot::Rover::level"), "carried by more than one object of this session (#"+other+", Depot::garage::front)")
	wants(t, run(t, s, "%state #"+front), `exhibited by object #`+front+` of "Depot::garage::front"`)
	wants(t, run(t, s, "%features #"+front), "Instance: Depot::garage::front (ID: "+front+")")
	wants(t, run(t, s, "%state #"+other), `exhibited by object #`+other+"\n")
}
