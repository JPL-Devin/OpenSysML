package runtime

import (
	"errors"
	"strings"
	"testing"
)

// MaterializationErrors reads the slots a caller would otherwise leave lazy, so
// an object created without reading it still reports what its defaults are.
func TestMaterializationErrorsReadsEverySlot(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub {
				attribute volume : Real = 2.0;
				attribute wrong : Real[3] = 1.0;
			}
			part def Holder {
				part sub : Sub;
				attribute two : Real = (1.0, 2.0);
			}
		}
	`)

	errs := ctx.MaterializationErrors(inst)
	if len(errs) != 2 {
		t.Fatalf("MaterializationErrors = %v, want the object's own and its nested object's", errs)
	}
	for _, want := range []string{"Holder.two", "Sub.wrong"} {
		if !containsError(errs, want) {
			t.Errorf("no error naming %s:\n%v", want, errs)
		}
	}
	for _, err := range errs {
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("err = %v, want ErrMultiplicityViolation", err)
		}
	}
}

// An object whose slots all materialize reports nothing, and one holding itself
// terminates rather than descending forever.
func TestMaterializationErrorsOfAConformingObject(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Node {
				attribute mass : Real = 1.0;
				part child : Node;
			}
			part def Holder {
				part root : Node;
				attribute total : Real[0..*] = root.mass;
			}
		}
	`)

	if errs := ctx.MaterializationErrors(inst); len(errs) != 0 {
		t.Errorf("MaterializationErrors = %v, want none", errs)
	}
	if errs := ctx.MaterializationErrors(nil); errs != nil {
		t.Errorf("MaterializationErrors(nil) = %v, want none", errs)
	}
}

func containsError(errs []error, substr string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), substr) {
			return true
		}
	}
	return false
}
