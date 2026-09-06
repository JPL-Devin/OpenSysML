package runtime

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// derivedInvalidationModel: values written with `=` over other features, read
// directly, transitively, through a binding and through a `default null`
// collection a classifier's subsetter later supersedes.
const derivedInvalidationModel = `
	package test {
		private import ScalarValues::*;
		private import SI::*;
		private import ISQ::*;
		private import NumericalFunctions::*;
		private import ControlFunctions::*;

		part def MassedComponent {
			part subcomponents : MassedComponent [*] default null;
			attribute mass :> ISQ::mass;
			attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);
		}
		part def Leaf :> MassedComponent { attribute :>> mass = 100 [kg]; }
		part def Stack :> MassedComponent {
			attribute :>> mass = 10 [kg];
			part a : Leaf :> subcomponents;
		}
		part def Bare :> MassedComponent { attribute :>> mass = 10 [kg]; }
		part def Loaded :> Bare { part a : Leaf :> subcomponents; }

		part def Box {
			attribute a : Integer default 3;
			attribute d : Integer = a * 2;
			attribute dd : Integer = d + 1;
		}
		part def Widget { attribute n : Integer = 1; }
		part def Rig {
			part w : Widget;
			attribute shown : Integer;
			bind shown = w.n;
			attribute twice : Integer = shown * 2;
		}
		part def Loop { attribute x : Integer = x + 1; }
		part def Rod {
			attribute len :> ISQ::length default 1 [m];
			attribute alt :> ISQ::length = 100 [cm];
			attribute twice :> ISQ::length = len * 2;
		}
		part def Host {
			attribute a : Integer default 3;
			attribute d : Integer = a + writer.n;
		}
		part def Writer {
			attribute n : Integer = 1;
			perform action set { action go { assign host.a := 9; } first go; }
		}
		part def Bin {
			attribute weights [*] default null;
			attribute grams :> ISQ::mass [*] = (1 [g])->select {in x; false};
			attribute kilos :> ISQ::mass [*] = (1 [kg])->select {in x; false};
			attribute total = sum(weights);
		}

		part box : Box;
		part rod : Rod;
		part bin : Bin;
		part host : Host;
		part writer : Writer;
		part stack : Stack;
		part bare : Bare;
		part rig : Rig;
		part loop : Loop;
	}
`

func readFormatted(t *testing.T, ctx *Context, inst *Instance, name string) string {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	return FormatValue(fv.HeldValue())
}

func setInt(t *testing.T, ctx *Context, inst *Instance, name string, n int64) {
	t.Helper()
	if err := inst.SetFeatureValue(ctx, name, constInt(n)); err != nil {
		t.Fatalf("SetFeatureValue(%s, %d): %v", name, n, err)
	}
}

// TestDerivedValueRecomputesWhenWhatItReadChanges: a value written with `=` is
// derived again, lazily, once a feature it read is written — transitively, so a
// value derived from a derived one follows too.
func TestDerivedValueRecomputesWhenWhatItReadChanges(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	box := instantiateNamed(t, ctx, idx, "test::box")
	if d, dd := readInt(t, ctx, box, "d"), readInt(t, ctx, box, "dd"); d != 6 || dd != 7 {
		t.Fatalf("d, dd = %d, %d before the write, want 6, 7", d, dd)
	}

	setInt(t, ctx, box, "a", 9)
	if d := box.FeatureValues["d"]; d.Materialized {
		t.Fatalf("d is still materialized as %s after a was written", FormatValue(d.HeldValue()))
	}
	if dd := box.FeatureValues["dd"]; dd.Materialized {
		t.Fatalf("dd is still materialized as %s after a was written", FormatValue(dd.HeldValue()))
	}
	if d, dd := readInt(t, ctx, box, "d"), readInt(t, ctx, box, "dd"); d != 18 || dd != 19 {
		t.Fatalf("d, dd = %d, %d after a := 9, want 18, 19", d, dd)
	}

	// Reading dd first derives d on the way; the write still reaches both.
	setInt(t, ctx, box, "a", 4)
	if dd, d := readInt(t, ctx, box, "dd"), readInt(t, ctx, box, "d"); d != 8 || dd != 9 {
		t.Fatalf("dd, d = %d, %d after a := 4, want 9, 8", dd, d)
	}
}

// TestWrittenDerivedValueKeepsWhatWasAssigned: a derived value a run assigned
// holds the assignment and is no longer derived, while what read it follows the
// assignment.
func TestWrittenDerivedValueKeepsWhatWasAssigned(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	box := instantiateNamed(t, ctx, idx, "test::box")
	if d, dd := readInt(t, ctx, box, "d"), readInt(t, ctx, box, "dd"); d != 6 || dd != 7 {
		t.Fatalf("d, dd = %d, %d, want 6, 7", d, dd)
	}

	setInt(t, ctx, box, "d", 100)
	if dd := readInt(t, ctx, box, "dd"); dd != 101 {
		t.Fatalf("dd = %d after d := 100, want 101", dd)
	}
	setInt(t, ctx, box, "a", 1)
	if d := box.FeatureValues["d"]; !d.Materialized || !d.Written {
		t.Fatalf("the write to a unmaterialized the assigned d (materialized %t, written %t)", d.Materialized, d.Written)
	}
	if d, dd := readInt(t, ctx, box, "d"), readInt(t, ctx, box, "dd"); d != 100 || dd != 101 {
		t.Fatalf("d, dd = %d, %d after a := 1, want the assigned 100 and 101", d, dd)
	}
}

// TestWriteOfTheSameValueLeavesDerivedValues: writing what a feature already
// holds changes nothing it was read for.
func TestWriteOfTheSameValueLeavesDerivedValues(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	box := instantiateNamed(t, ctx, idx, "test::box")
	readInt(t, ctx, box, "dd")
	setInt(t, ctx, box, "a", 3)
	if d, dd := box.FeatureValues["d"], box.FeatureValues["dd"]; !d.Materialized || !dd.Materialized {
		t.Fatalf("writing a's own value unmaterialized d (%t) or dd (%t)", d.Materialized, dd.Materialized)
	}
	setInt(t, ctx, box, "a", 5)
	if dd := readInt(t, ctx, box, "dd"); dd != 11 {
		t.Fatalf("dd = %d after a := 5, want 11", dd)
	}
}

// TestWriteOfAnEqualQuantityInAnotherUnitRecomputesDerivedValues: a quantity
// equal in magnitude but stated in another unit is a change to what read it, as
// an operation over it carries the unit into its result.
func TestWriteOfAnEqualQuantityInAnotherUnitRecomputesDerivedValues(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	rod := instantiateNamed(t, ctx, idx, "test::rod")
	if got := readFormatted(t, ctx, rod, "twice"); got != "2 [m]" {
		t.Fatalf("twice = %s before the write, want 2 [m]", got)
	}
	alt, err := rod.GetFeatureValue(ctx, "alt")
	if err != nil {
		t.Fatalf("GetFeatureValue(alt): %v", err)
	}
	if err := rod.SetFeatureValue(ctx, "len", alt.HeldValue()); err != nil {
		t.Fatalf("SetFeatureValue(len, 100 [cm]): %v", err)
	}
	if twice := rod.FeatureValues["twice"]; twice.Materialized {
		t.Fatalf("twice is still materialized as %s after len was restated in cm", FormatValue(twice.HeldValue()))
	}
	if got := readFormatted(t, ctx, rod, "twice"); got != "200 [cm]" {
		t.Fatalf("twice = %s after len := 100 [cm], want 200 [cm]", got)
	}
	// The same quantity in the same unit is no change.
	if err := rod.SetFeatureValue(ctx, "len", alt.HeldValue()); err != nil {
		t.Fatalf("SetFeatureValue(len, 100 [cm]) again: %v", err)
	}
	if twice := rod.FeatureValues["twice"]; !twice.Materialized {
		t.Fatal("writing len's own value again unmaterialized twice")
	}
}

// TestWriteOfAnEmptyCollectionInAnotherUnitRecomputesDerivedValues: an empty
// collection measures its elements in a unit, which the zero its sum yields is in.
func TestWriteOfAnEmptyCollectionInAnotherUnitRecomputesDerivedValues(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	bin := instantiateNamed(t, ctx, idx, "test::bin")
	if got := readFormatted(t, ctx, bin, "total"); got != "0" {
		t.Fatalf("total = %s before any write, want 0", got)
	}
	emptyIn := func(name, unit string) Value {
		fv, err := bin.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if got, ok := fv.HeldValue().Sequence().ElementUnit(); !ok || got.Text != unit {
			t.Fatalf("%s = %s measured in %q, %t; want an empty sequence in %s", name, FormatValue(fv.HeldValue()), got.Text, ok, unit)
		}
		return fv.HeldValue()
	}
	grams, kilos := emptyIn("grams", "g"), emptyIn("kilos", "kg")
	for _, step := range []struct {
		val  Value
		want string
	}{{grams, "0 [g]"}, {kilos, "0 [kg]"}} {
		if err := bin.SetFeatureValue(ctx, "weights", step.val); err != nil {
			t.Fatalf("SetFeatureValue(weights): %v", err)
		}
		if got := readFormatted(t, ctx, bin, "total"); got != step.want {
			t.Fatalf("total = %s after weights := %s, want %s", got, FormatValue(step.val), step.want)
		}
	}
	if err := bin.SetFeatureValue(ctx, "weights", kilos); err != nil {
		t.Fatalf("SetFeatureValue(weights) again: %v", err)
	}
	if total := bin.FeatureValues["total"]; !total.Materialized {
		t.Fatal("writing weights' own value again unmaterialized total")
	}
}

// TestWriteUnderADerivationDerivesItAgain: a read a derivation makes starts a
// behavior (here, of the occurrence a package-level part denotes) that writes a
// value the derivation read before; its result is derived over again, not committed.
func TestWriteUnderADerivationDerivesItAgain(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	host := instantiateNamed(t, ctx, idx, "test::host")
	if d, a := readInt(t, ctx, host, "d"), readInt(t, ctx, host, "a"); d != 10 || a != 9 {
		t.Fatalf("d, a = %d, %d after the first read of d, want 10, 9: writer's run wrote a under d", d, a)
	}
	if len(ctx.deriving) != 0 {
		t.Fatalf("%d derivations still under way", len(ctx.deriving))
	}
	if a, d := host.FeatureValues["a"], host.FeatureValues["d"]; !slices.Contains(a.dependents, d) {
		t.Fatal("d is no longer listed as a dependent of a")
	}
	setInt(t, ctx, host, "a", 20)
	if d := readInt(t, ctx, host, "d"); d != 21 {
		t.Fatalf("d = %d after a := 20, want 21", d)
	}
}

// TestClassifierSubsetterRecomputesWhatReadTheFoldedDefault: a derived value
// read while a `default null` collection was folded follows the subsetter a
// classifier later brings to that collection.
func TestClassifierSubsetterRecomputesWhatReadTheFoldedDefault(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	bare := instantiateNamed(t, ctx, idx, "test::bare")
	if got := readFormatted(t, ctx, bare, "totalMass"); got != "10 [kg]" {
		t.Fatalf("totalMass = %s before classifying, want 10 [kg]", got)
	}
	if err := ctx.classify(bare, idx.LookupQualified("test::Loaded")[0]); err != nil {
		t.Fatalf("classify(Loaded): %v", err)
	}
	subs, err := bare.GetFeatureValue(ctx, "subcomponents")
	if err != nil {
		t.Fatalf("GetFeatureValue(subcomponents): %v", err)
	}
	if got := len(elementsOf(subs.HeldValue())); got != 1 {
		t.Fatalf("subcomponents holds %d members after classifying, want the subsetter", got)
	}
	if got := readFormatted(t, ctx, bare, "totalMass"); got != "110 [kg]" {
		t.Fatalf("totalMass = %s after classifying, want 110 [kg]", got)
	}
}

// TestDerivedValueFollowsAWriteThroughAPartAndABinding: a rollup follows a write
// into a part it read through, and a value read through a binding follows a
// write to the binding's other end.
func TestDerivedValueFollowsAWriteThroughAPartAndABinding(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))

	stack := instantiateNamed(t, ctx, idx, "test::stack")
	if got := readFormatted(t, ctx, stack, "totalMass"); got != "110 [kg]" {
		t.Fatalf("stack.totalMass = %s, want 110 [kg]", got)
	}
	leaf := readInstance(t, ctx, stack, "a")
	heavy, err := ctx.EvalWithScopeOn(parseExpr(t, "300 [kg]"), idx.LookupQualified("test")[0].Scope, stack)
	if err != nil {
		t.Fatalf("300 [kg]: %v", err)
	}
	if err := leaf.SetFeatureValue(ctx, "mass", heavy); err != nil {
		t.Fatalf("SetFeatureValue(a.mass): %v", err)
	}
	if got := readFormatted(t, ctx, stack, "totalMass"); got != "310 [kg]" {
		t.Fatalf("stack.totalMass = %s after a.mass := 300 [kg], want 310 [kg]", got)
	}

	rig := instantiateNamed(t, ctx, idx, "test::rig")
	if twice := readInt(t, ctx, rig, "twice"); twice != 2 {
		t.Fatalf("twice = %d, want 2", twice)
	}
	w := readInstance(t, ctx, rig, "w")
	setInt(t, ctx, w, "n", 5)
	if twice := readInt(t, ctx, rig, "twice"); twice != 10 {
		t.Fatalf("twice = %d after w.n := 5, want 10", twice)
	}
	// A re-read of the bound feature that changes nothing derives nothing again.
	readInt(t, ctx, rig, "shown")
	if twice := rig.FeatureValues["twice"]; !twice.Materialized {
		t.Fatal("re-reading shown, unchanged, unmaterialized twice")
	}
}

// TestProbeRollbackRestoresDerivedValuesAndTheirDependencies: a probe that
// writes what a derived value read is undone whole — the derived value reads as
// before, and a later write still reaches it.
func TestProbeRollbackRestoresDerivedValuesAndTheirDependencies(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	box := instantiateNamed(t, ctx, idx, "test::box")
	if dd := readInt(t, ctx, box, "dd"); dd != 7 {
		t.Fatalf("dd = %d, want 7", dd)
	}
	a, d, dd := box.FeatureValues["a"], box.FeatureValues["d"], box.FeatureValues["dd"]
	aDeps, dDeps := len(a.dependents), len(d.dependents)

	end := ctx.beginProbe()
	setInt(t, ctx, box, "a", 9)
	if got := readInt(t, ctx, box, "dd"); got != 19 {
		t.Fatalf("dd = %d in the probe, want 19", got)
	}
	end()

	if !d.Materialized || !dd.Materialized {
		t.Fatalf("the probe left d (materialized %t) or dd (%t) unmaterialized", d.Materialized, dd.Materialized)
	}
	if got, gotDD := readInt(t, ctx, box, "d"), readInt(t, ctx, box, "dd"); got != 6 || gotDD != 7 {
		t.Fatalf("d, dd = %d, %d after the probe, want 6, 7", got, gotDD)
	}
	if len(a.dependents) != aDeps || len(d.dependents) != dDeps {
		t.Fatalf("the probe left a with %d dependents and d with %d, want %d and %d", len(a.dependents), len(d.dependents), aDeps, dDeps)
	}
	setInt(t, ctx, box, "a", 2)
	if got, gotDD := readInt(t, ctx, box, "d"), readInt(t, ctx, box, "dd"); got != 4 || gotDD != 5 {
		t.Fatalf("d, dd = %d, %d after a := 2, want 4, 5", got, gotDD)
	}
}

// TestProbeRollbackForgetsDependenciesRecordedInIt: a derived value first read
// in a probe is unmaterialized with it, and what it read no longer lists it.
func TestProbeRollbackForgetsDependenciesRecordedInIt(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	box := instantiateNamed(t, ctx, idx, "test::box")
	a := box.FeatureValues["a"]

	end := ctx.beginProbe()
	if dd := readInt(t, ctx, box, "dd"); dd != 7 {
		t.Fatalf("dd = %d in the probe, want 7", dd)
	}
	end()

	if d, dd := box.FeatureValues["d"], box.FeatureValues["dd"]; d.Materialized || dd.Materialized {
		t.Fatalf("the probe left d (materialized %t) or dd (%t) behind", d.Materialized, dd.Materialized)
	}
	if len(a.dependents) != 0 {
		t.Fatalf("the probe left a with %d dependents", len(a.dependents))
	}
}

// TestSelfReferentialDerivedValueStaysACycle: a value derived from itself is a
// cycle on every read, before and after a write, and never hangs.
func TestSelfReferentialDerivedValueStaysACycle(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	loop := instantiateNamed(t, ctx, idx, "test::loop")
	for i := 0; i < 2; i++ {
		done := make(chan error, 1)
		go func() {
			_, err := loop.GetFeatureValue(ctx, "x")
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, ErrCyclicFeatureValue) {
				t.Fatalf("GetFeatureValue(x) error = %v, want ErrCyclicFeatureValue", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("GetFeatureValue(x) hung")
		}
		if deps := loop.FeatureValues["x"].dependents; len(deps) != 0 {
			t.Fatalf("x lists itself as a dependent (%d)", len(deps))
		}
		setInt(t, ctx, loop, "x", 1)
		if x := readInt(t, ctx, loop, "x"); x != 1 {
			t.Fatalf("x = %d after x := 1, want 1", x)
		}
		loop.FeatureValues["x"].Value, loop.FeatureValues["x"].Materialized, loop.FeatureValues["x"].Written = Value{}, false, false
	}
}

// TestNoDependencyIsRecordedOutsideADerivation: ordinary reads and writes record
// nothing, so an object with no derived value carries no edges.
func TestNoDependencyIsRecordedOutsideADerivation(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, derivedInvalidationModel))
	box := instantiateNamed(t, ctx, idx, "test::box")
	readInt(t, ctx, box, "a")
	setInt(t, ctx, box, "a", 4)
	readInt(t, ctx, box, "a")
	for name, fv := range box.FeatureValues {
		if fv.dependents != nil {
			t.Errorf("%s has dependents %v with nothing derived", name, fv.dependents)
		}
	}
	if len(ctx.deriving) != 0 {
		t.Fatalf("a derivation frame is still open: %d", len(ctx.deriving))
	}
}
