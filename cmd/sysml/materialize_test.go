package main

import (
	"strings"
	"testing"
)

// TestCheckReportsMaterializationDiagnostics checks the exit-status rule for the
// diagnostics only materializing an object produces: creating it is part of the
// run, so a default whose value count does not conform to its feature's
// multiplicity is reported and the run exits 2 — never `no errors` and 0.
func TestCheckReportsMaterializationDiagnostics(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name   string
		model  string
		object string
		status int
		want   []string
	}{
		{
			name: "a conforming default is materialized and the model reported clean",
			model: `package M {
    private import ScalarValues::Real;
    part def Sub { attribute volume : Real = 2.0; }
    part def Craft {
        part tank : Sub;
        attribute volumes : Real[1] = tank.volume;
    }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 0,
			want:   []string{"✓ Created instance of M::craft", "no errors"},
		},
		{
			name: "a default of fewer values than the declared lower bound is reported",
			model: `package M {
    private import ScalarValues::Real;
    part def Sub { attribute volume : Real = 2.0; }
    part def Craft {
        part tank : Sub;
        attribute volumes : Real[3] = tank.volume;
    }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 2,
			want:   []string{"slot craft.volumes", "1 value(s) bound to a feature with multiplicity lower bound 3"},
		},
		{
			name: "a multi-valued default on a feature declaring no multiplicity is held to 1..1",
			model: `package M {
    private import ScalarValues::Real;
    part def Craft { attribute volumes : Real = (1.0, 2.0); }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 2,
			want:   []string{"slot craft.volumes", "2 value(s) bound to a feature with multiplicity upper bound 1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := check(t, binary, tc.model, "-instantiate", tc.object, "-validate")
			wantReport(t, got, tc.status, tc.want...)
			if tc.status != 0 && strings.Contains(got.output(), "no errors") {
				t.Errorf("an invalid model was reported as having no errors:\n%s", got.output())
			}
			// A diagnostic about the model belongs on stderr, where the other
			// findings of a run that decided nothing are reported.
			if tc.status != 0 && strings.Contains(got.stdout, "multiplicity") {
				t.Errorf("a diagnostic was reported on stdout:\n%s", got.stdout)
			}
		})
	}
}

// TestCheckReportsMaterializationDiagnosticsAsJSON checks that -json carries the
// same finding as data, so a script reads why the run was undecided rather than
// matching printed output.
func TestCheckReportsMaterializationDiagnosticsAsJSON(t *testing.T) {
	binary := buildCLI(t)

	const model = `package M {
    private import ScalarValues::Real;
    part def Craft { attribute volumes : Real = (1.0, 2.0); }
    part craft : Craft;
}
`
	got := check(t, binary, model, "-instantiate", "M::craft", "-validate", "-json")
	if got.status != 2 {
		t.Errorf("exit status = %d, want 2\n%s", got.status, got.output())
	}
	for _, want := range []string{`"status": "unresolved"`, `"code": "runtime.materialize"`,
		"2 value(s) bound to a feature with multiplicity upper bound 1"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("the JSON report is missing %q:\n%s", want, got.stdout)
		}
	}
}
