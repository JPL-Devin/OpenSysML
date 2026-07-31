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
