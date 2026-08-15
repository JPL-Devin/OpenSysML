package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

// The did-you-mean hint belongs to the diagnostic, so every surface that shows
// an unresolved reference shows it: the CLI, the REPL and the LSP all read these
// messages.
func TestUnresolvedReferenceSuggestsSpelling(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    string
		absent  string
		noError bool
	}{
		{
			name: "unimported library name suggests its qualified spelling",
			src:  "part def A { attribute x : Integer = 1; }",
			want: "unresolved reference: Integer — did you mean ScalarValues::Integer?",
		},
		{
			name: "typo suggests the nearest name",
			src:  "part def A { attribute x : Intger; }",
			want: "did you mean ScalarValues::Integer",
		},
		{
			name: "typo of a library name declared elsewhere",
			src:  "part def A { attribute x : Whel; }",
			want: "did you mean ",
		},
		{
			name:   "a qualified name is not second-guessed",
			src:    "part def A { attribute x : Nowhere::Integer; }",
			want:   "unresolved reference: Nowhere::Integer",
			absent: "did you mean",
		},
		{
			name:    "an imported library name resolves",
			src:     "package T { private import ScalarValues::*; part def A { attribute x : Integer = 1; } }",
			noError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := NewWorkspace()
			ws.Open("t.sysml", []byte(tc.src), 1)

			var errs []string
			for _, d := range ws.Diagnostics("t.sysml") {
				if d.Severity == passes.SeverityError {
					errs = append(errs, d.Message)
				}
			}

			if tc.noError {
				if len(errs) > 0 {
					t.Fatalf("unexpected error(s): %s", strings.Join(errs, "; "))
				}
				return
			}

			joined := strings.Join(errs, "; ")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("diagnostics %q do not contain %q", joined, tc.want)
			}
			if tc.absent != "" && strings.Contains(joined, tc.absent) {
				t.Errorf("diagnostics %q contain %q", joined, tc.absent)
			}
		})
	}
}
