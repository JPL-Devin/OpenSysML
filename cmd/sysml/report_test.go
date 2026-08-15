package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/repl"
)

// TestUndecidedVerdictTakesTheCommandPrefix checks the boundary the command
// reports a verdict through: whatever the prompt renders a check it could not
// make as, the command reports it under its own prefix once, on stderr.
func TestUndecidedVerdictTakesTheCommandPrefix(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  []string
	}{{
		name:  "a line the prompt prefixes is restated in the command's prefix",
		lines: []string{"error: unresolved reference: Nope"},
		want:  []string{"sysml: unresolved reference: Nope"},
	}, {
		name:  "every line of the verdict is restated, not only the first",
		lines: []string{"error: calc invocation failed: Fall", "error: unbound parameter: b"},
		want:  []string{"sysml: calc invocation failed: Fall", "sysml: unbound parameter: b"},
	}, {
		name:  "a line locating a finding in the source keeps its own shape",
		lines: []string{"model.sysml:1:42: error: unresolved reference: Nope"},
		want:  []string{"model.sysml:1:42: error: unresolved reference: Nope"},
	}, {
		name:  "a line that says error of something else keeps saying it",
		lines: []string{"the error: state was never reached"},
		want:  []string{"sysml: the error: state was never reached"},
	}, {
		name:  "a check the prompt states with no prefix still takes the command's",
		lines: []string{"no satisfaction assertion in the session"},
		want:  []string{"sysml: no satisfaction assertion in the session"},
	}, {
		name:  "the same check phrased as the prompt phrases it reads the same",
		lines: []string{"error: no satisfaction assertion in the session"},
		want:  []string{"sysml: no satisfaction assertion in the session"},
	}, {
		name:  "what a run printed before it stopped is not restated",
		lines: []string{"✓ action Mission::Descend started", "error: action stopped at Falling without completing"},
		want:  []string{"✓ action Mission::Descend started", "sysml: action stopped at Falling without completing"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			r := &reporter{out: &out, err: &errs}
			r.verdict(repl.Verdict{Subject: "Nope", Status: repl.VerdictUnresolved, Lines: tc.lines})

			want := strings.Join(tc.want, "\n") + "\n"
			if errs.String() != want {
				t.Errorf("stderr = %q, want %q", errs.String(), want)
			}
			if out.String() != "" {
				t.Errorf("a verdict that was never decided should not reach stdout, got %q", out.String())
			}
		})
	}
}
