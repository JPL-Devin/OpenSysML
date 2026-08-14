package runtime

import "testing"

const redefinedCollectionModel = `
	package test {
		private import ScalarValues::Real;
		private import RealFunctions::*;
		part def Sub { attribute mass : Real; }
		part def System {
			part Subsystems : Sub[*];
			attribute totalmass : Real = sum(Subsystems.mass);
		}
		part def Sat :> System {
			part subsystems : Sub[*] :>> Subsystems;
		}
		part sat : Sat {
			part bus : Sub :> subsystems { attribute :>> mass = 4.0; }
			part sensor : Sub :> subsystems { attribute :>> mass = 3.0; }
		}
	}
`

// TestRedefinedCollectionReadsSubsetsUnderEitherName pins that a collection
// shared by a redefinition holds the parts subsetting it however it is reached:
// the subsetting parts name the redefining feature, not the name read here.
func TestRedefinedCollectionReadsSubsetsUnderEitherName(t *testing.T) {
	for _, first := range []string{"Subsystems", "subsystems"} {
		t.Run(first, func(t *testing.T) {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, redefinedCollectionModel))
			matches := idx.LookupQualified("test::sat")
			if len(matches) != 1 {
				t.Fatalf("test::sat: %d matching symbols, want 1", len(matches))
			}
			inst, err := ctx.Instantiate(matches[0])
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			// Reading the redefined collection is what materializes the shared
			// slot, so each name has to be the one read first in turn.
			for _, name := range []string{first, "Subsystems", "subsystems"} {
				slot, err := inst.GetSlot(ctx, name)
				if err != nil {
					t.Fatalf("GetSlot(%s): %v", name, err)
				}
				if got := len(elementsOf(slot.HeldValue())); got != 2 {
					t.Fatalf("%s: %d elements, want 2", name, got)
				}
			}
			slot, err := inst.GetSlot(ctx, "totalmass")
			if err != nil {
				t.Fatalf("GetSlot(totalmass): %v", err)
			}
			if got := FormatTraceValue(slot.HeldValue()); got != "7.0" {
				t.Fatalf("totalmass = %s, want 7.0", got)
			}
		})
	}
}
