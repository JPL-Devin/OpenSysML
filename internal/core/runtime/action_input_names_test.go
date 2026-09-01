package runtime

import (
	"errors"
	"strings"
	"testing"
)

// inputNamesModel declares one input parameter and one attribute, the two names
// an input may bind.
const inputNamesModel = `action bump {
	in attribute step = 1;
	attribute result = 0;
	first start;
	action inner { assign result := result + step; }
	done;
	succession first start then inner;
	succession first inner then done;
}`

// TestUnknownActionInputIsReported: an input naming no feature of the action is
// reported, not bound into the feature space and answered as an output.
func TestUnknownActionInputIsReported(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, inputNamesModel)
	ctx := NewContext(model, resolver, 1000)
	bump := resolveSymbol(t, root, "bump")

	outputs, err := ctx.ExecuteActionWithInputs(bump, map[string]Value{"nope": constInt(7)})
	if !errors.Is(err, ErrUnknownActionInput) {
		t.Fatalf("outputs = %v, err = %v; want ErrUnknownActionInput", outputs, err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error = %v, want it to name the input", err)
	}
}

// TestDeclaredActionInputsBind: a parameter and a plain attribute both name a
// feature the caller may seed.
func TestDeclaredActionInputsBind(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, inputNamesModel)
	ctx := NewContext(model, resolver, 1000)
	bump := resolveSymbol(t, root, "bump")

	outputs, err := ctx.ExecuteActionWithInputs(bump, map[string]Value{
		"step":   constInt(4),
		"result": constInt(10),
	})
	if err != nil {
		t.Fatalf("ExecuteActionWithInputs: %v", err)
	}
	if got := outputs["result"]; got.Const.Int != 14 {
		t.Fatalf("result = %+v, want 14", got)
	}
}
