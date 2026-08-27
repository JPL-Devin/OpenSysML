package main

import (
	"strings"
	"testing"
)

// unsetFeatureValueModel declares a valueless attribute of a library value type beside a
// valued one, a collection of them, and a part typed by a class.
const unsetFeatureValueModel = `package P {
    private import ScalarValues::*;
    part def Engine { attribute power : Real = 120.0; }
    part def Q {
        attribute d : Real;
        attribute ds : Real[2];
        attribute k : Real = 2.0;
        part engine : Engine;
    }
}
`

// TestUnsetFeatureValueReadsTheSameOnEverySurface checks that a feature value that holds no value
// reads as <unset> wherever a value is reported — the feature value listing, an
// evaluation, and the JSON a script reads — while a valued feature value and an object
// still read as what they hold.
func TestUnsetFeatureValueReadsTheSameOnEverySurface(t *testing.T) {
	binary := buildCLI(t)

	t.Run("a piped feature value listing reports it unset", func(t *testing.T) {
		got := runPiped(t, binary, "%instantiate P::Q\n%features P::Q\n", unsetFeatureValueModel)
		if got.status != exitHolds {
			t.Errorf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
		}
		for _, want := range []string{"d = <unset>", "ds = [<unset>, <unset>]", "k = 2.0", "engine = Instance(ID: "} {
			if !strings.Contains(got.output(), want) {
				t.Errorf("the feature value listing is missing %q:\n%s", want, got.output())
			}
		}
		// The object materialized for a valueless value-typed feature is not
		// reported as an object with no features.
		if strings.Contains(got.output(), "(no features)") {
			t.Errorf("a feature value that holds no value was reported as an empty object:\n%s", got.output())
		}
	})

	t.Run("an evaluation reports it unset", func(t *testing.T) {
		got := check(t, binary, unsetFeatureValueModel, "-instantiate", "P::Q", "-e", "P::Q::d", "-e", "P::Q::k")
		wantReport(t, got, exitHolds, "= <unset>", "= 2.0")
	})

	t.Run("the JSON report carries the same spelling", func(t *testing.T) {
		got := check(t, binary, unsetFeatureValueModel, "-instantiate", "P::Q", "-e", "P::Q::d", "-json")
		if got.status != exitHolds {
			t.Errorf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
		}
		// JSON escapes the angle brackets, so the value is matched as encoded.
		if !strings.Contains(got.stdout, `\u003cunset\u003e`) {
			t.Errorf("the JSON report does not carry the unset value:\n%s", got.stdout)
		}
	})
}
