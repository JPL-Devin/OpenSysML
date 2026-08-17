package repl

import (
	"strings"
	"testing"
)

// unsetSlotModel declares valueless features of value types — a library one, an
// attribute definition with no features, and a collection — beside a valued
// attribute and objects of classes.
const unsetSlotModel = `package P {
    private import ScalarValues::*;
    attribute def Empty;
    attribute def Point { attribute x : Real = 1.0; }
    part def Engine;
    part def Q {
        attribute d : Real;
        attribute ds : Real[2];
        attribute empty : Empty;
        attribute origin : Point;
        attribute k : Real = 2.0;
        part engine : Engine;
    }
}
`

// A valueless feature of a value type holds no value, so the slot listing says
// so rather than naming the object materialization holds for it. What does hold
// a value — a valued attribute, a value type with features, an object of a class
// — still reads as what it holds.
func TestSlotListingReportsAValuelessValueTypedFeatureAsUnset(t *testing.T) {
	s := loadSource(t, unsetSlotModel)
	wants(t, run(t, s, "%instantiate P::Q"), "Created instance")

	slots := run(t, s, "%slots P::Q")
	wants(t, slots,
		"d = <unset>",
		"ds = [<unset>, <unset>]",
		"empty = <unset>",
		"k = 2.00",
		"origin = Instance(ID: ",
		"x = 1.00",
		"engine = Instance(ID: ",
	)
	// The object materialization holds for such a feature is not named, and not
	// expanded — only the class-typed part is reported as an empty object.
	rejects(t, slots, "d = Instance(", "empty = Instance(")
	if n := strings.Count(slots, "(no features)"); n != 1 {
		t.Errorf("%d empty objects reported, want only the class-typed part:\n%s", n, slots)
	}
}

// An evaluation of the same feature reports the same thing the slot listing
// does: the two surfaces read one runtime value.
func TestEvaluationReportsAValuelessValueTypedFeatureAsUnset(t *testing.T) {
	s := loadSource(t, unsetSlotModel)
	wants(t, run(t, s, "%instantiate P::Q"), "Created instance")

	wants(t, run(t, s, "%eval P::Q::d"), "= <unset>")
	wants(t, run(t, s, "%eval P::Q::k"), "= 2.00")
}
