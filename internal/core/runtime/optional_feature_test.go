package runtime

import (
	"errors"
	"testing"
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
