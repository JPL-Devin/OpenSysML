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
