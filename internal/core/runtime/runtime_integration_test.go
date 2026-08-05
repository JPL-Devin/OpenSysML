package runtime

import (
	"testing"
)

func TestIntegration_ParseAndInstantiate(t *testing.T) {
	// End-to-end: parse → validate → instantiate → verify default value
	src := `
		part def Wheel {
			attribute diameter: Real = 0.5;
		}
	`

	model, resolver, rootScope := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 100000)

	wheelSym := resolveSymbol(t, rootScope, "Wheel")
	inst, err := ctx.Instantiate(wheelSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	slot, err := inst.GetSlot(ctx, "diameter")
	if err != nil {
		t.Fatalf("GetSlot failed: %v", err)
	}

	if slot.Value.Const.Real != 0.5 {
		t.Errorf("expected diameter=0.5, got %v", slot.Value.Const.Real)
	}
}
