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
