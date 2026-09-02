package opensysml_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const complexSource = `package C {
	private import ScalarValues::*;
	private import ComplexFunctions::*;
	part def Signal {
		attribute z : Complex = rect(1.5, -2.0);
		attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
	}
	action conj {
		in z : Complex;
		out w : Complex;
		first start;
		action inner { assign w := rect(re(z), -im(z)); }
		done;
		succession first start then inner;
		succession first inner then done;
	}
}`

// A Complex is one value of its own type in every direction, over every
// transport: never a Real, never a Sequence of two.
func TestComplexIsOneValueOverEveryTransport(t *testing.T) {
	address := startService(t)
	for name, client := range map[string]opensysml.Client{
		"in-process":    newClient(t),
		"connect-proto": dialClient(t, address),
		"connect-json":  dialClient(t, address, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			info, err := client.ServerInfo(ctx)
			if err != nil {
				t.Fatalf("ServerInfo: %v", err)
			}
			if !info.Has(opensysml.CapabilityComplexValues) {
				t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityComplexValues)
			}
			model := parse(t, client, complexSource)

			value, err := client.Evaluate(ctx, model, "ComplexFunctions::rect(1.5, -2.0)")
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if value != opensysml.Complex(complex(1.5, -2)) {
				t.Errorf("Evaluate = %#v, want Complex(1.5 - 2.0i)", value)
			}

			instantiation, err := client.Instantiate(ctx, model, "C::Signal")
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			root := instantiation.Root
			if got := root.FeatureValues["z"].Value; got != opensysml.Complex(complex(1.5, -2)) {
				t.Errorf("z = %#v, want Complex(1.5 - 2.0i)", got)
			}
			want := []opensysml.Value{opensysml.Complex(complex(1, 2)), opensysml.Complex(complex(3, 4))}
			if got := root.FeatureValues["zs"].Values; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
				t.Errorf("zs = %#v, want %#v", got, want)
			}

			run, err := client.ExecuteAction(ctx, model, "C::conj",
				map[string]opensysml.Value{"z": opensysml.Complex(complex(2, 5))})
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if got := run.Outputs["w"]; got != opensysml.Complex(complex(2, -5)) {
				t.Errorf("w = %#v, want Complex(2.0 - 5.0i)", got)
			}
		})
	}
}

func TestComplexRendersInRectangularForm(t *testing.T) {
	for _, testcase := range []struct {
		value opensysml.Complex
		want  string
	}{
		{opensysml.Complex(complex(1.5, -2)), "1.5 - 2.0i"},
		{opensysml.Complex(complex(1, 2)), "1.0 + 2.0i"},
		{opensysml.Complex(complex(0, 0)), "0.0 + 0.0i"},
		{opensysml.Complex(complex(-3, 0)), "-3.0 + 0.0i"},
	} {
		if got := fmt.Sprintf("%v", testcase.value); got != testcase.want {
			t.Errorf("%v renders as %q, want %q", complex128(testcase.value), got, testcase.want)
		}
	}
}
