package repl

import (
	"errors"
	"strconv"
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

	// A nested object of the definition counts once it exists, under the path
	// reaching it. The error does not create it: the part stays unread, and its
	// machine's entry action unrun, until something asks for it.
	run(t, s, "%instantiate Fleet::driver")
	got = run(t, s, "%state Fleet::Rover::modes Fleet::rover")
	wants(t, got, `object #`+defOnly+` of "Fleet::Rover"`, "or name Fleet::Rover to address it")
	if strings.Contains(got, "Fleet::driver::r") {
		t.Errorf("the error names a part nothing has read:\n%s", got)
	}
	wants(t, run(t, s, "%invoke Fleet::rover bump"), `no instance of the usage "Fleet::rover"`)
	if fv := s.instances["Fleet::driver"].FeatureValues["r"]; fv.Materialized || fv.Written {
		t.Errorf("the failed lookup materialized Fleet::driver.r: %+v", fv)
	}

	wants(t, run(t, s, "%features Fleet::driver.r"), `log = "W"`)
	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::rover"), `of "Fleet::driver::r"`, "or name Fleet::Rover or Fleet::driver::r to address one of them")

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
// names once they exist, by identity since no path reaches one, and a feature they
// all carry is a question between them. Unread, the error does not create them.
func TestNotInstantiatedNamesCollectionMembersByIdentity(t *testing.T) {
	s := loadFixture(t, "testdata/collection_machine.sysml")
	run(t, s, "%instantiate Depot::garage")
	wants(t, run(t, s, "%state Depot::Rover::modes Depot::Rover"), `error: no instance of "Depot::Rover" (use %instantiate first)`)
	if fv := s.instances["Depot::garage"].FeatureValues["bays"]; fv.Materialized {
		t.Fatalf("the failed lookup materialized Depot::garage.bays: %+v", fv)
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

// After a second %instantiate of a name, the object it superseded is not
// addressable by identity on any command: the session no longer holds it, even
// though the runtime keeps it until the next rebuild. The current object is.
func TestSupersededObjectIsNotAddressableById(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	first := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	wants(t, run(t, s, "%features #"+first), "Instance: Fleet::rover (ID: "+first+")")

	again := run(t, s, "%instantiate Fleet::rover")
	second := objectIDIn(t, again)
	if second == first {
		t.Fatalf("the second %%instantiate reused object #%s", first)
	}
	wants(t, again, "note: Fleet::rover now denotes this object; object #"+first+" is no longer named")

	_, _, err := s.objectRef("#" + first)
	var nerr *NoObjectError
	if !errors.As(err, &nerr) || strconv.FormatInt(nerr.ID, 10) != first || !nerr.Superseded {
		t.Errorf("#%s: got %v, want a superseded NoObjectError", first, err)
	}
	gone := "no object #" + first + " in this session: it was superseded"
	wants(t, run(t, s, "%features #"+first), "error:", gone)
	wants(t, run(t, s, "%invoke #"+first+" bump"), "error:", gone)
	wants(t, run(t, s, "%state #"+first), "error:", gone)
	wants(t, run(t, s, "%state Fleet::Rover::modes #"+first), "error:", gone)

	wants(t, run(t, s, "%features #"+second), "Instance: Fleet::rover (ID: "+second+")", "level = 10")
	wants(t, run(t, s, "%invoke #"+second+" bump"), "✓")
	wants(t, run(t, s, "%features Fleet::rover"), "level = 11")
	wants(t, run(t, s, "%state #"+second), `exhibited by object #`+second+` of "Fleet::rover"`)
}

// The objects a current root's materialized features hold stay addressable by
// identity, members of a multi-valued part included; the check reads no feature
// that has not been materialized already.
func TestHeldObjectsStayAddressableById(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")
	driver := s.instances["Fleet::driver"]
	if fv := driver.FeatureValues["r"]; fv != nil && fv.Materialized {
		t.Fatal("r was materialized by %instantiate alone")
	}
	if _, _, err := s.objectRef("#99"); err == nil {
		t.Error("#99 reached an object")
	}
	if fv := driver.FeatureValues["r"]; fv != nil && fv.Materialized {
		t.Error("looking up an identity materialized r")
	}

	nested := objectIDIn(t, run(t, s, "%state Fleet::driver.r"))
	wants(t, run(t, s, "%features #"+nested), "Instance: Fleet::driver::r (ID: "+nested+")")

	c := loadFixture(t, "testdata/collection_machine.sysml")
	run(t, c, "%instantiate Depot::garage")
	listing := run(t, c, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	member := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])
	wants(t, run(t, c, "%features #"+member), "Instance: #"+member+" (ID: "+member+")")
	wants(t, run(t, c, "%state #"+member), `exhibited by object #`+member, "Current state: waiting")
}

// A debugging session over the superseded object ends with the %instantiate that
// superseded it, and the next debugging command says why; one over the object a
// different name denotes is untouched.
func TestSupersededObjectEndsItsDebuggingSession(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	first := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	wants(t, run(t, s, "%state Fleet::rover"), `Debugging state machine "modes"`, "Current state: waiting")

	again := run(t, s, "%instantiate Fleet::rover")
	wants(t, again, `note: state debugging session for "Fleet::rover" ended (object #`+first+
		" performing it was superseded by this instance of Fleet::rover)")
	if s.stateExec != nil {
		t.Error("the session over the superseded object is still running")
	}
	wants(t, run(t, s, "%advance 5"), "error:", "no active state machine session",
		`ended when a second %instantiate Fleet::rover superseded the object #`+first+" performing it")

	// The action debugger is ended the same way.
	run(t, s, "%instantiate Fleet::driver")
	wants(t, run(t, s, "%action Fleet::Rover::bump Fleet::driver.r"), "Started action executor")
	kept := run(t, s, "%instantiate Fleet::rover")
	rejects(t, kept, "debugging session")
	if s.actionExec == nil {
		t.Fatal("instantiating an unrelated name ended the action session")
	}
	ended := run(t, s, "%instantiate Fleet::driver")
	wants(t, ended, `note: action debugging session for "Fleet::Rover::bump" ended (object #`,
		"performing it was superseded by this instance of Fleet::driver)")
	if s.actionExec != nil {
		t.Error("the action session over the superseded nested object is still running")
	}
	wants(t, run(t, s, "%step"), "error:", "no active action session",
		"ended when a second %instantiate Fleet::driver superseded the object #")
}

// Every object the session holds is addressable by id, however many there are:
// the bound on a subject search does not apply to what is already materialized.
// A debugging session over a member beyond that bound survives an unrelated
// second %instantiate.
func TestEveryHeldObjectIsAddressableById(t *testing.T) {
	s := loadFixture(t, "testdata/large_collection.sysml")
	run(t, s, "%instantiate Farm::field")
	rows := collectionIDs(t, run(t, s, "%features Farm::field"), "rows")
	if len(rows) != 3 {
		t.Fatalf("rows holds %d members, want 3", len(rows))
	}
	held := 2 + len(rows) // field, spare and the rows
	var last string
	for _, row := range rows {
		cells := collectionIDs(t, run(t, s, "%features #"+row), "cells")
		held += len(cells)
		last = cells[len(cells)-1]
	}
	if held <= carrierLimit {
		t.Fatalf("the session holds %d objects, want more than %d", held, carrierLimit)
	}

	wants(t, run(t, s, "%features #"+last), "Instance: #"+last+" (ID: "+last+")", "count = 0")
	wants(t, run(t, s, "%invoke #"+last+" tick"), "✓ Invoked tick on object #"+last)
	wants(t, run(t, s, "%features #"+last), "count = 1")

	wants(t, run(t, s, "%action Farm::Cell::tick #"+last), "Started action executor")
	run(t, s, "%instantiate Farm::spare")
	again := run(t, s, "%instantiate Farm::spare")
	wants(t, again, "is no longer named")
	rejects(t, again, "debugging session")
	if s.actionExec == nil {
		t.Fatal("a second %instantiate of another name ended the session over a held member")
	}
	wants(t, run(t, s, "%continue"), "✓ Action completed")
	wants(t, run(t, s, "%features #"+last), "count = 2")
}

// %state <machine>, for a machine one held object exhibits, drives that object's
// running performance: the do action its timer re-runs writes the object's own
// slots, the ones %features shows under the identity %instances lists.
func TestStateOverAMachineNamedAloneDrivesItsOneExhibitor(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_timer.sysml")
	id := objectIDIn(t, run(t, s, "%instantiate TA::Sys"))

	got := run(t, s, "%state lp")
	wants(t, got, `Debugging state machine "lp" exhibited by object #`+id+` of "TA::Sys"`, "Current state: run")
	rejects(t, got, "Started state machine executor")

	wants(t, run(t, s, "%advance 2.5 [s]"), "Advanced to 2.5 (2 event(s) processed)")
	wants(t, run(t, s, "%instances"), "TA::Sys (ID: "+id+")")
	wants(t, run(t, s, "%features #"+id), "x = 0.6000000000000001", "n = 3")

	// The qualified name of the machine attaches the same way.
	wants(t, run(t, s, "%state TA::Sys::lp"), `exhibited by object #`+id+` of "TA::Sys"`)
}

// %state <machine> <object>, for the machine the object exhibits, attaches to the
// running performance: its do action is not started a second time on the object,
// and the events the session dispatches write the object's slots.
func TestStateOverTheExhibitedMachineAndObjectDoesNotDoubleRun(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_timer.sysml")
	id := objectIDIn(t, run(t, s, "%instantiate TA::Sys"))
	wants(t, run(t, s, "%features #"+id), "x = 0.2", "n = 1")

	got := run(t, s, "%state lp #"+id)
	wants(t, got, `exhibited by object #`+id, "note:", "attaches to that running machine")
	rejects(t, got, "Started state machine executor")
	wants(t, run(t, s, "%features #"+id), "x = 0.2", "n = 1")
	wants(t, run(t, s, "%events"), "1 events")

	wants(t, run(t, s, "%advance 2.5 [s]"), "2 event(s) processed")
	wants(t, run(t, s, "%features #"+id), "x = 0.6000000000000001", "n = 3")
}

// A machine a type exhibits that no held object runs is refused rather than
// performed detached from any object, and the refusal names both forms that
// address an object.
func TestStateOverAMachineNoObjectExhibitsRefuses(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_timer.sysml")

	_, err := s.startStateMachine("lp", nil)
	var eerr *ExhibitorsError
	if !errors.As(err, &eerr) {
		t.Fatalf("got %v, want an ExhibitorsError", err)
	}
	if eerr.Machine != "lp" || strings.Join(eerr.Types, ",") != "TA::Sys" || len(eerr.Objects) != 0 {
		t.Errorf("got %+v, want lp of TA::Sys exhibited by no object", eerr)
	}
	if s.stateExec != nil {
		t.Errorf("a detached performance was started: %q", s.stateExec.fqn)
	}

	got := run(t, s, "%state TA::Sys::lp")
	wants(t, got, "error:", `no object of this session exhibits "TA::Sys::lp"`, `object of "TA::Sys"`,
		"%instantiate", "%state <object>", "%state TA::Sys::lp <object>")
	rejects(t, got, "Started state machine executor", "Debugging state machine")

	// Once an object exhibits it, the same command attaches.
	id := objectIDIn(t, run(t, s, "%instantiate TA::Sys"))
	wants(t, run(t, s, "%state TA::Sys::lp"), `exhibited by object #`+id)
}

// A machine several held objects exhibit is refused, naming every one of them,
// rather than attached to whichever was found first; naming an object attaches.
func TestStateOverAMachineSeveralObjectsExhibitRefuses(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	rover := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	run(t, s, "%instantiate Fleet::driver")
	nested := objectIDIn(t, run(t, s, "%features Fleet::driver.r"))

	_, err := s.startStateMachine("Fleet::Rover::modes", nil)
	var eerr *ExhibitorsError
	if !errors.As(err, &eerr) {
		t.Fatalf("got %v, want an ExhibitorsError", err)
	}
	want := []ObjectRef{{ID: atoi(t, rover), Name: "Fleet::rover"}, {ID: atoi(t, nested), Name: "Fleet::driver::r"}}
	if len(eerr.Objects) != len(want) || eerr.Objects[0] != want[0] || eerr.Objects[1] != want[1] {
		t.Errorf("got objects %+v, want %+v", eerr.Objects, want)
	}
	if s.stateExec != nil {
		t.Errorf("a machine was attached to despite the ambiguity: %q", s.stateExec.selfFQN)
	}

	got := run(t, s, "%state Fleet::Rover::modes")
	wants(t, got, "error:", `2 objects of this session exhibit "Fleet::Rover::modes"`,
		`#`+nested+` of "Fleet::driver::r"`, `#`+rover+` of "Fleet::rover"`, "%state <object>", "%state Fleet::Rover::modes <object>")
	rejects(t, got, "Started state machine executor", "Debugging state machine")

	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::driver.r"), `exhibited by object #`+nested, "note:")
	wants(t, run(t, s, "%state #"+rover), `exhibited by object #`+rover+` of "Fleet::rover"`)
}

// A definition a type exhibits through usages typed by it is refused named alone
// before any object is held, naming that type rather than running detached; once
// one held object exhibits it as two usages, it is as ambiguous named alone as it
// is with the object named.
func TestStateOverASharedDefinitionNamedAlone(t *testing.T) {
	s := loadFixture(t, "testdata/shared_machine.sysml")

	_, err := s.startStateMachine("Shared::Blink", nil)
	var eerr *ExhibitorsError
	if !errors.As(err, &eerr) {
		t.Fatalf("got %v, want an ExhibitorsError", err)
	}
	if eerr.Machine != "Shared::Blink" || strings.Join(eerr.Types, ",") != "Shared::Lamp" || len(eerr.Objects) != 0 {
		t.Errorf("got %+v, want Shared::Blink of Shared::Lamp exhibited by no object", eerr)
	}
	if s.stateExec != nil {
		t.Errorf("a detached performance was started: %q", s.stateExec.fqn)
	}
	got := run(t, s, "%state Shared::Blink")
	wants(t, got, "error:", `no object of this session exhibits "Shared::Blink"`, `object of "Shared::Lamp"`, "%state Shared::Blink <object>")
	rejects(t, got, "Started state machine executor")

	run(t, s, "%instantiate Shared::lamp")
	_, err = s.startStateMachine("Shared::Blink", nil)
	var aerr *AmbiguousMachineError
	if !errors.As(err, &aerr) {
		t.Fatalf("got %v, want an AmbiguousMachineError", err)
	}
	if strings.Join(aerr.Usages, ",") != "Shared::Lamp::front,Shared::Lamp::rear" {
		t.Errorf("got %+v, want both exhibited usages of Shared::Blink", aerr)
	}
	wants(t, run(t, s, "%state Shared::Blink"), "error:", `exhibits "Shared::Blink" as 2 machines`, "Shared::Lamp::front or Shared::Lamp::rear")

	wants(t, run(t, s, "%state Shared::Lamp::rear"), `Debugging state machine "rear" exhibited by object #`, "Current state: dark")
	wants(t, run(t, s, "%advance 2"), "Current state: lit")
	wants(t, run(t, s, "%features Shared::lamp"), "front", "dark", "rear", "lit")
}

// A definition no type exhibits still runs detached when named alone, since no
// object's performance of it exists to attach to.
func TestStateOverAnUnexhibitedDefinitionRunsDetached(t *testing.T) {
	s := loadFixture(t, "testdata/performed_machine.sysml")
	wants(t, run(t, s, "%state Two::Check"), "Started state machine executor", "Current state: checking")
	wants(t, run(t, s, "%advance 5"), "Current state: checked")
}

// atoi reads an object identity a report printed.
func atoi(t *testing.T, digits string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		t.Fatalf("%q is no object identity: %v", digits, err)
	}
	return id
}

// collectionIDs reads the identities a %features listing shows feature holding.
func collectionIDs(t *testing.T, listing, feature string) []string {
	t.Helper()
	at := strings.Index(listing, feature+" = [")
	if at < 0 {
		t.Fatalf("%s holds no collection:\n%s", feature, listing)
	}
	members := listing[at+len(feature)+4:]
	members = members[:strings.Index(members, "]")]
	var ids []string
	for _, m := range strings.Split(members, ",") {
		ids = append(ids, objectIDIn(t, m))
	}
	return ids
}
