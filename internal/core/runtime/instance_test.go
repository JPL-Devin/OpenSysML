package runtime

import (
	"testing"
)

func TestInstantiate_SimplePartDef(t *testing.T) {
	src := `
		part def Wheel {
			attribute diameter: Real = 0.5;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	wheelSym := resolveSymbol(t, root, "Wheel")

	inst, err := ctx.Instantiate(wheelSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	if inst.ID != 1 {
		t.Errorf("expected ID=1, got %d", inst.ID)
	}

	if len(inst.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(inst.Slots))
	}

	diameterSlot, ok := inst.Slots["diameter"]
	if !ok {
		t.Fatal("expected 'diameter' slot")
	}

	// Check default value evaluated
	if diameterSlot.Value.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", diameterSlot.Value.Kind)
	}
	if diameterSlot.Value.Const.Real != 0.5 {
		t.Errorf("expected Real=0.5, got %f", diameterSlot.Value.Const.Real)
	}
}

// An unbounded upper bound carries the numeric value 0, so a feature declared
// [*] must not be mistaken for a scalar and filled with a single default.
func TestInstantiate_UnboundedDefaultIsNotAScalar(t *testing.T) {
	src := `
		part def Wheel {
			attribute diameter: Real = 0.5;
		}
		part def Car {
			part wheels: Wheel[0..*] = 1;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Car"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	slot, err := inst.GetSlot(ctx, "wheels")
	if err != nil {
		t.Fatalf("GetSlot failed: %v", err)
	}
	if slot.Value.Kind != ValInvalid {
		t.Errorf("unbounded slot holds a scalar value %v", slot.Value)
	}
	if slot.Values.Kind != ValSequence {
		t.Errorf("expected a sequence, got %v", slot.Values.Kind)
	}
}

func TestInstantiate_IDAllocation(t *testing.T) {
	src := `part def A {}`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	aSym := resolveSymbol(t, root, "A")

	inst1, _ := ctx.Instantiate(aSym)
	inst2, _ := ctx.Instantiate(aSym)

	if inst1.ID == inst2.ID {
		t.Error("expected unique IDs")
	}
}

func TestGetSlot_LazyComposite(t *testing.T) {
	src := `
		part def Engine {}
		part def Car {
			part engine: Engine;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	carSym := resolveSymbol(t, root, "Car")
	inst, err := ctx.Instantiate(carSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	// Verify engine slot NOT materialized initially
	engineSlot := inst.Slots["engine"]
	if engineSlot == nil {
		t.Fatal("expected engine slot")
	}
	if engineSlot.Materialized {
		t.Error("expected engine slot NOT materialized after Instantiate")
	}

	// Call GetSlot → should lazy-materialize
	slot, err := inst.GetSlot(ctx, "engine")
	if err != nil {
		t.Fatalf("GetSlot failed: %v", err)
	}

	if !slot.Materialized {
		t.Error("expected engine slot materialized after GetSlot")
	}

	if slot.Value.Kind != ValInstance {
		t.Errorf("expected ValInstance, got %v", slot.Value.Kind)
	}

	// Verify child instance exists in registry
	childInst, ok := ctx.getInstance(slot.Value.Instance)
	if !ok {
		t.Error("expected child instance registered")
	}
	if childInst.Type.Name != "Engine" {
		t.Errorf("expected Engine type, got %s", childInst.Type.Name)
	}
}

// A multi-valued feature holds its default's contents, not <unknown>: a single
// value written on it is the collection's one element.
func TestMultiValuedDefaultMaterializes(t *testing.T) {
	src := `
		part def Rig {
			attribute mass = 100.0;
			attribute doubles[0..*] = mass * 2.0;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Rig"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	slot, err := inst.GetSlot(ctx, "doubles")
	if err != nil {
		t.Fatalf("GetSlot failed: %v", err)
	}
	if slot.Values.Kind != ValSequence {
		t.Fatalf("Values.Kind = %v, want a sequence", slot.Values.Kind)
	}
	elements := slot.Values.Sequence.Elements()
	if len(elements) != 1 || elements[0].Const.Real != 200.0 {
		t.Errorf("doubles = %v, want [200]", elements)
	}
}

// A multi-valued feature that is typed as well as given a default holds the
// default's contents rather than an instantiation of its type.
func TestTypedMultiValuedDefaultHoldsItsContents(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub { attribute volume : Real; }
			part def Ctx {
				part subsystem : Sub[0..*];
				attribute volumes : Real[0..*] = subsystem.volume;
			}
			part ctx : Ctx {
				part a : Sub :> subsystem { attribute :>> volume = 2.0; }
				part b : Sub :> subsystem { attribute :>> volume = 3.5; }
			}
		}
	`))
	matches := idx.LookupQualified("test::ctx")
	if len(matches) != 1 {
		t.Fatalf("test::ctx: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	slot, err := inst.GetSlot(ctx, "volumes")
	if err != nil {
		t.Fatalf("GetSlot(volumes): %v", err)
	}
	if got := FormatTraceValue(slot.HeldValue()); got != "(2.0, 3.5)" {
		t.Errorf("volumes = %s, want (2.0, 3.5)", got)
	}
}

// A nested part usage with a body of its own is an object shaped by that body:
// what it redeclares must win over what its type declares.
func TestNestedUsageBodyOverridesItsType(t *testing.T) {
	src := `
		part def Engine {
			attribute power = 200.0;
		}
		part def Car {
			part engine : Engine {
				attribute power redefines Engine::power = 250.0;
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	car, err := ctx.Instantiate(resolveSymbol(t, root, "Car"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	if got := nestedReal(t, ctx, car, "engine", "power"); got != 250.0 {
		t.Errorf("engine.power = %v, want 250 (the usage body's value)", got)
	}
}

// A written default takes precedence over instantiation in GetSlot, so the
// feature holds that value and is not something to materialize an object from.
func TestCompositeTypeOfIgnoresDefaultedFeature(t *testing.T) {
	src := `
		attribute def Temp {
			attribute v = 1.0;
		}
		part def Gauge {
			attribute plain : Temp;
			attribute written : Temp = 5.0;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	features := ctx.FeaturesOf(resolveSymbol(t, root, "Gauge"))
	for i := range features {
		feat := &features[i]
		composite := ctx.CompositeTypeOf(feat)
		if feat.Name == "written" && composite != nil {
			t.Errorf("written holds 5.0, but reports %s as composite", composite.Name)
		}
		if feat.Name == "plain" && composite == nil {
			t.Error("plain is materialized from Temp, want it reported as composite")
		}
	}
}

// An untyped nested part is still an object: its body is its shape, so it
// materializes and its members hold values.
func TestUntypedNestedPartMaterializes(t *testing.T) {
	src := `
		part def Car {
			attribute mass = 1500.0;
			part engine {
				attribute power = 300.0;
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	car, err := ctx.Instantiate(resolveSymbol(t, root, "Car"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	if got := nestedReal(t, ctx, car, "engine", "power"); got != 300.0 {
		t.Errorf("engine.power = %v, want 300", got)
	}
}

// nestedReal reads a Real out of the instance held by one of inst's slots.
func nestedReal(t *testing.T, ctx *Context, inst *Instance, slotName, nestedName string) float64 {
	t.Helper()
	slot, err := inst.GetSlot(ctx, slotName)
	if err != nil {
		t.Fatalf("GetSlot(%q) failed: %v", slotName, err)
	}
	if slot.Value.Kind != ValInstance {
		t.Fatalf("slot %q holds %v, want a nested instance", slotName, slot.Value.Kind)
	}
	nested, ok := ctx.Instance(slot.Value.Instance)
	if !ok {
		t.Fatalf("slot %q references unknown instance %d", slotName, slot.Value.Instance)
	}
	nestedSlot, err := nested.GetSlot(ctx, nestedName)
	if err != nil {
		t.Fatalf("GetSlot(%q) failed: %v", nestedName, err)
	}
	return nestedSlot.Value.Const.Real
}
