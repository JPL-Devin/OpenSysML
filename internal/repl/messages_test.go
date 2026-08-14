package repl

import (
	"strings"
	"testing"
)

// TestMessageQuality pins the exact text of the diagnostics the prompt reports,
// since the wording, the position and the remedy are the interface.
func TestMessageQuality(t *testing.T) {
	cases := []struct {
		name  string
		decls string
		line  string
		want  []string
	}{{
		// A real failure of an expression of literals is the answer, so an empty
		// session reports it rather than "no declarations loaded".
		name: "division_by_zero_without_declarations",
		line: "%eval 1/0",
		want: []string{"error: evaluation failed: division by zero"},
	}, {
		name:  "division_by_zero_with_declarations",
		decls: "part def Wheel;",
		line:  "%eval 1/0",
		want:  []string{"error: evaluation failed: division by zero"},
	}, {
		name: "unresolved_name_without_declarations",
		line: "%eval missing",
		want: []string{"error: no declarations loaded (literals work, but feature references need declarations)"},
	}, {
		// One parser diagnostic, about the expression, with the caret treatment
		// declarations get — not the recovery fallout of a namespace member.
		name: "incomplete_expression",
		line: "%eval 1 +",
		want: []string{"error: 1:4: expected an expression", "1 +", "   ^"},
	}, {
		name:  "incomplete_expression_with_declarations",
		decls: "part def Wheel;",
		line:  "%eval 1 +",
		want:  []string{"error: 1:4: expected an expression", "1 +", "   ^"},
	}, {
		name: "operand_type_mismatch",
		line: `%eval 1 + "a"`,
		want: []string{
			`error: 1:1: type mismatch: operator '+' is not defined for an Integer and a string`,
			`1 + "a"`,
			"^~~~~~~",
		},
	}, {
		name:  "operand_type_mismatch_with_declarations",
		decls: "attribute d = 1;",
		line:  `%eval d + "a"`,
		want: []string{
			`error: 1:1: type mismatch: operator '+' is not defined for an Integer and a string`,
			`d + "a"`,
			"^~~~~~~",
		},
	}, {
		// A mismatch written in a calc keeps the wrapped message that names the
		// calc, rather than a position in the line the user typed.
		name:  "operand_type_mismatch_inside_a_calc",
		decls: "calc def f { in n; return n + \"a\"; }",
		line:  "%eval f(1)",
		want:  []string{`error: evaluation failed: calc f: `, `operator '+' is not defined for`},
	}, {
		// A name of another kind names its kind in SysML terms, never a Go type.
		name:  "calc_argument_of_wrong_kind",
		decls: "part def Wheel;",
		line:  "%calc Wheel 1",
		want:  []string{"error: not a calc: Wheel is a part def, not a calc definition or usage"},
	}, {
		// A usage error, not a verdict: the model is not what is wrong.
		name:  "constraint_argument_of_wrong_kind",
		decls: "part def Wheel;",
		line:  "%constraint Wheel",
		want:  []string{"error: not a constraint: Wheel is a part def, not a constraint definition or usage"},
	}, {
		name:  "requirement_argument_of_wrong_kind",
		decls: "part def Wheel;",
		line:  "%requirement Wheel",
		want:  []string{"error: not a requirement: Wheel is a part def, not a requirement definition or usage"},
	}, {
		// One wording for a name nothing declares, the parser's and the runtime's.
		name:  "unknown_name",
		decls: "part def Wheel;",
		line:  "%eval nope",
		want:  []string{"error: unresolved reference: nope"},
	}, {
		name:  "unknown_qualified_name",
		decls: "package P { part def Wheel; }",
		line:  "%instantiate P::Nope",
		want:  []string{"error: unresolved reference: P::Nope"},
	}, {
		// The remedy of the other surface is named too, so the two agree.
		name:  "save_to_an_unknown_format",
		decls: "part def Wheel;",
		line:  "%save /tmp/systemica-message-test.txt",
		want: []string{
			`error: cannot tell the format of "/tmp/systemica-message-test.txt": expected .sysml, .kerml or .ttl, ` +
				"so name the file with a .sysml, .kerml or .ttl extension, or pass -convert on the command line",
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSession()
			if tc.decls != "" {
				if res := s.Submit(tc.decls); len(res.Diagnostics) > 0 {
					t.Fatalf("declarations have diagnostics: %v", res.Diagnostics)
				}
			}
			got := run(t, s, tc.line)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q in output:\n%s", want, got)
				}
			}
			if strings.Contains(got, "*ast.") {
				t.Errorf("output names a Go type:\n%s", got)
			}
		})
	}
}

// TestLoadMissingFileReportsPathOnce pins the unwrapped %load failure: the
// operation and the path are each named once.
func TestLoadMissingFileReportsPathOnce(t *testing.T) {
	s := NewSession()
	_, _, err := s.RunMeta("%load /nonexistent/model.sysml")
	if err == nil {
		t.Fatal("expected an error loading a missing file")
	}
	want := "cannot read /nonexistent/model.sysml: no such file or directory"
	if err.Error() != want {
		t.Errorf("err = %q; want %q", err.Error(), want)
	}
	if strings.Count(err.Error(), "/nonexistent/model.sysml") != 1 {
		t.Errorf("path named more than once: %q", err.Error())
	}
}
