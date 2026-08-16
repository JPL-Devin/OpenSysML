package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
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

	errs, bounded := ctx.MaterializationErrors(inst)
	if bounded {
		t.Error("bounded = true, want a small object read in full")
	}
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

	if errs, _ := ctx.MaterializationErrors(inst); len(errs) != 0 {
		t.Errorf("MaterializationErrors = %v, want none", errs)
	}
	if errs, _ := ctx.MaterializationErrors(nil); errs != nil {
		t.Errorf("MaterializationErrors(nil) = %v, want none", errs)
	}
}

// A redefinition names the redefined feature again and the two names read one
// slot, so one faulty slot is reported once.
func TestMaterializationErrorsReportsARedefinedSlotOnce(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Base { attribute mass : Real; }
			part def Holder :> Base {
				attribute grossMass :>> mass = (1.0, 2.0);
			}
		}
	`)

	errs, _ := ctx.MaterializationErrors(inst)
	if len(errs) != 1 {
		t.Fatalf("MaterializationErrors = %v, want the shared slot reported once", errs)
	}
	if !errors.Is(errs[0], ErrMultiplicityViolation) {
		t.Errorf("err = %v, want ErrMultiplicityViolation", errs[0])
	}
}

// Nesting multiplies and reading a slot materializes the objects it holds, so a
// wide model stops at the walk's budget rather than allocating without end.
func TestMaterializationErrorsBoundsAWideModel(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Leaf { attribute v : Real = 1.0; }
			part def L3 { part leaves : Leaf[100]; }
			part def L2 { part inner : L3[100]; }
			part def L1 { part inner : L2[100]; }
			part def Holder { part inner : L1[100]; }
		}
	`)

	bounded := make(chan bool, 1)
	go func() {
		_, hit := ctx.MaterializationErrors(inst)
		bounded <- hit
	}()
	select {
	case hit := <-bounded:
		if !hit {
			t.Error("bounded = false, want the walk to report it stopped at its budget")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("MaterializationErrors did not return, want a bounded walk")
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
