package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// TestCalcConditionalExpressionWithBodyCondition covers the `if` a calculation
// body may start with: a conditional expression whose condition holds a body
// expression must still be read as an expression, not as an if statement.
func TestCalcConditionalExpressionWithBodyCondition(t *testing.T) {
	bodies := []struct {
		name string
		body string
	}{
		{"returned conditional", "return if n > 0 ? 1 else 0;"},
		{"returned conditional over a body expression", "return if xs->exists { it > n } ? 1 else 0;"},
		{"implicit result", "if n > 0 ? 1 else 0"},
		{"implicit result over a body expression", "if xs->exists { it > n } ? 1 else 0"},
		{"implicit result over parentheses", "if (n > 0 and (n < 9)) ? 1 else 0"},
		{"body expression in an if statement", "if xs->exists { it > n } { return : Integer = 1; } return : Integer = 0;"},
	}

	for _, tc := range bodies {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P { calc def C { in n : Integer; in xs : Integer[*]; " + tc.body + " } }"
			p := New(source.New("t.sysml", []byte(src)))
			p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics for %q: %v", tc.body, p.Diagnostics)
			}
		})
	}
}
