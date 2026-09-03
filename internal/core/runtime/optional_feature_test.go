package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// An optional composite feature fills to its lower bound like a collection:
// `part spare : Wheel[0..1]` holds no object of its own, reads as empty, and
// holds one only through a feature that subsets it.
func TestOptionalScalarFeatureHoldsOnlyContributions(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Car {
			part spare : Wheel[0..1];
			part wheels : Wheel[0..*];
			part trailer : Wheel[0..1];
			part hitch : Wheel[1] :> trailer;
		}
	}`))
	car, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"spare", "wheels"} {
		fv, err := car.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("car.%s: %v", name, err)
		}
		if held := fv.HeldValue(); held.Kind == ValInstance || elementCount(&held) != 0 {
			t.Errorf("car.%s = %s, want no object", name, FormatValue(held))
		}
		read, err := fv.ReadValue(name)
		if err != nil || elementCount(&read) != 0 {
			t.Errorf("car.%s reads %s, %v; want the empty sequence", name, FormatValue(read), err)
		}
	}
	trailer, err := car.GetFeatureValue(ctx, "trailer")
	if err != nil {
		t.Fatalf("car.trailer: %v", err)
	}
	hitch := objectAt(t, ctx, car, "hitch")
	if held := trailer.HeldValue(); held.Kind != ValInstance || held.Instance != hitch.ID {
		t.Errorf("car.trailer = %s, want the hitch that subsets it (%d)", FormatValue(held), hitch.ID)
	}
	if n := len(ctx.instances); n != 2 {
		t.Errorf("instantiating Car made %d objects, want the car and its hitch", n)
	}
}

// A required abstract feature holds only what subsets it, so one nothing subsets
// violates its lower bound rather than holding an empty value; enough
// contributions satisfy it, scalar or collection alike.
func TestRequiredAbstractFeatureDemandsContributions(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Bare {
			abstract part axle : Wheel[1];
			abstract part wheels : Wheel[2..*];
		}
		part def Fitted {
			abstract part axle : Wheel[1];
			part front : Wheel[1] :> axle;
			abstract part wheels : Wheel[2..*];
			part left : Wheel[1] :> wheels;
			part right : Wheel[1] :> wheels;
		}
	}`))
	bare, err := ctx.Instantiate(oneSymbol(t, idx, "test::Bare"))
	if err != nil {
		t.Fatalf("instantiate Bare: %v", err)
	}
	for _, name := range []string{"axle", "wheels"} {
		fv, err := bare.GetFeatureValue(ctx, name)
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("bare.%s = %v, %v; want %v", name, fv, err, ErrMultiplicityViolation)
		}
	}
	fitted, err := ctx.Instantiate(oneSymbol(t, idx, "test::Fitted"))
	if err != nil {
		t.Fatalf("instantiate Fitted: %v", err)
	}
	axle, err := fitted.GetFeatureValue(ctx, "axle")
	if err != nil {
		t.Fatalf("fitted.axle: %v", err)
	}
	if held, front := axle.HeldValue(), objectAt(t, ctx, fitted, "front"); held.Kind != ValInstance || held.Instance != front.ID {
		t.Errorf("fitted.axle = %s, want the front wheel (%d)", FormatValue(held), front.ID)
	}
	wheels, err := fitted.GetFeatureValue(ctx, "wheels")
	if err != nil {
		t.Fatalf("fitted.wheels: %v", err)
	}
	if held := wheels.HeldValue(); elementCount(&held) != 2 {
		t.Errorf("fitted.wheels = %s, want the two wheels subsetting it", FormatValue(held))
	}
}

// An abstract feature declaring no multiplicity is bound by what it subsets, as
// `abstract action decisions :> controls` is by `controls[0..*]`; a concrete one stays 1..1.
func TestAbstractFeatureInheritsMultiplicityFromWhatItSubsets(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Car {
			part wheels : Wheel[0..*];
			abstract part spares :> wheels;
			part one : Wheel :> wheels;
		}
		part def Bare {
			abstract part fitted : Wheel[1..*];
			abstract part needed :> fitted;
		}
	}`))
	car, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	spares, err := car.GetFeatureValue(ctx, "spares")
	if err != nil {
		t.Fatalf("car.spares: %v", err)
	}
	if held := spares.HeldValue(); spares.Feature.Scalar() || elementCount(&held) != 0 {
		t.Errorf("car.spares = %s (scalar %t), want an empty collection", FormatValue(held), spares.Feature.Scalar())
	}
	one := objectAt(t, ctx, car, "one")
	wheels, err := car.GetFeatureValue(ctx, "wheels")
	if err != nil {
		t.Fatalf("car.wheels: %v", err)
	}
	if held := wheels.HeldValue(); elementCount(&held) != 1 || held.Sequence().Elements()[0].Instance != one.ID {
		t.Errorf("car.wheels = %s, want the one wheel (%d)", FormatValue(held), one.ID)
	}
	bare, err := ctx.Instantiate(oneSymbol(t, idx, "test::Bare"))
	if err != nil {
		t.Fatalf("instantiate Bare: %v", err)
	}
	if _, err := bare.GetFeatureValue(ctx, "needed"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("bare.needed: err = %v, want %v through fitted's [1..*]", err, ErrMultiplicityViolation)
	}
}

// A valueless optional declaration evaluates to the empty sequence whether it is
// named bare or qualified; a required one keeps its no-value error both ways.
func TestValuelessOptionalDeclarationEvaluatesEmptyHoweverSpelled(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def P { attribute tags : String[0..*]; attribute mass : Real; }
		attribute names : String[0..*];
		attribute weight : Real;
	}`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	for _, src := range []string{"names", "test::names", "P::tags", "test::P::tags"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("%s = %s, %v; want the empty sequence", src, FormatValue(val), err)
		}
	}
	for _, src := range []string{"weight", "test::weight", "P::mass", "test::P::mass"} {
		if val, err := evalIn(t, ctx, pkg.Scope, src); err == nil {
			t.Errorf("%s = %s, want a no-value error", src, FormatValue(val))
		}
	}
}

// A declaration bound to an optional multiplicity by what it redefines or, if
// abstract, subsets evaluates as it does on an object: empty, bare or qualified.
func TestInheritedOptionalDeclarationEvaluatesEmpty(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Wheel;
		part def Car {
			part wheels : Wheel[0..*];
			abstract part spares :> wheels;
			part fitted : Wheel[1..*];
			abstract part needed :> fitted;
		}
		part def Van :> Car {
			part :>> wheels;
		}
		part wheels : Wheel[0..*];
		abstract part spares :> wheels;
	}`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	before := len(ctx.instances)
	for _, src := range []string{"spares", "test::spares", "Car::spares", "test::Car::spares", "Van::wheels"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("%s = %s, %v; want the empty sequence", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "spares istype Wheel"); err != nil || val.Kind != ValConst || !val.Const.Bool {
		t.Errorf("spares istype Wheel = %s, %v; want true of nothing", FormatValue(val), err)
	}
	if made := len(ctx.instances) - before; made != 0 {
		t.Errorf("reading the inherited optional declarations made %d object(s), want none", made)
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "Car::needed"); err == nil {
		t.Errorf("Car::needed = %s, want an error through fitted's [1..*]", FormatValue(val))
	}
}

// A feature whose lower bound is infinite admits no count, so holding nothing
// does not read as empty.
func TestInfiniteLowerBoundDoesNotReadAsEmpty(t *testing.T) {
	fv := &FeatureValue{Feature: &EffectiveFeature{Name: "p", Multiplicity: semantics.Range{
		Lower: semantics.Bound{Known: true, Infinite: true},
		Upper: semantics.Bound{Known: true, Infinite: true},
	}}}
	if read, err := fv.ReadValue("p"); !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Errorf("reading unset [*..*] p: %s, %v; want %v", FormatValue(read), err, ErrUninitializedFeatureValue)
	}
}

// A required feature holding nothing is still uninitialized when read.
func TestRequiredUnsetFeatureReadsAsUninitialized(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def P { attribute mass : Real; attribute tags : String[0..*]; }
	}`))
	p, err := ctx.Instantiate(oneSymbol(t, idx, "test::P"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	mass, err := p.GetFeatureValue(ctx, "mass")
	if err != nil {
		t.Fatalf("p.mass: %v", err)
	}
	if _, err := mass.ReadValue("mass"); !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Errorf("reading unset mass: err = %v, want %v", err, ErrUninitializedFeatureValue)
	}
	tags, err := p.GetFeatureValue(ctx, "tags")
	if err != nil {
		t.Fatalf("p.tags: %v", err)
	}
	if read, err := tags.ReadValue("tags"); err != nil || elementCount(&read) != 0 {
		t.Errorf("reading unset tags: %s, %v; want the empty sequence", FormatValue(read), err)
	}
}

// An optional part or item declaration names no object of its own: read directly,
// bare or qualified, it is the empty sequence — as through an instantiated owner.
func TestOptionalOccurrenceDeclarationEvaluatesEmpty(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Wheel { attribute radius : Real = 0.3; }
		item def Tag;
		part def Car {
			part spare : Wheel[0..1];
			item label : Tag[0..1];
			part fitted : Wheel;
		}
		part spare : Wheel[0..1];
		item label : Tag[0..1];
		part fitted : Wheel;
	}`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	before := len(ctx.instances)
	for _, src := range []string{"spare", "test::spare", "label", "test::label", "Car::spare", "test::Car::label", "spare.radius"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("%s = %s, %v; want the empty sequence", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "spare istype Wheel"); err != nil || val.Kind != ValConst || !val.Const.Bool {
		t.Errorf("spare istype Wheel = %s, %v; want true of nothing", FormatValue(val), err)
	}
	if made := len(ctx.instances) - before; made != 0 {
		t.Errorf("reading the optional declarations made %d object(s), want none", made)
	}
	for _, src := range []string{"fitted", "test::fitted"} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || val.Kind != ValInstance {
			t.Errorf("%s = %s, %v; want the one object", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "fitted.radius"); err != nil || val.Kind != ValConst || val.Const.Real != 0.3 {
		t.Errorf("fitted.radius = %s, %v; want 0.3", FormatValue(val), err)
	}
	car, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
	if err != nil {
		t.Fatalf("instantiate Car: %v", err)
	}
	for _, name := range []string{"spare", "label"} {
		fv, err := car.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("car.%s: %v", name, err)
		}
		if read, err := fv.ReadValue(name); err != nil || elementCount(&read) != 0 {
			t.Errorf("car.%s reads %s, %v; want the empty sequence", name, FormatValue(read), err)
		}
	}
}
