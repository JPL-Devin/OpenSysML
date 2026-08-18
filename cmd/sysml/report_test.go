package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// TestUndecidedVerdictTakesTheCommandPrefix checks the boundary the command
// reports a verdict through: whatever the prompt renders a check it could not
// make as, the command reports it under its own prefix, on stderr.
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
		name:  "a line that says error of something else is not a prefix",
		lines: []string{"the error: state was never reached"},
		want:  []string{"the error: state was never reached"},
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
