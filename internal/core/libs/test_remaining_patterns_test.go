package libs

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Test remaining high-impact patterns
func TestRemainingPatterns(t *testing.T) {
	tests := []struct{
		name string
		input string
	}{
		// Pattern 1: succession with multiplicity before name
		{
			name: "succession_multiplicity_first",
			input: `
				package Test {
					succession [1] mySuccession first [1] a then [1] b;
				}
			`,
		},
		// Pattern 2: connector with multiplicity before name
		{
			name: "connector_multiplicity_first",
			input: `
				package Test {
					connector [1] myConn from [1] a to [1] b;
				}
			`,
		},
		// Pattern 3: portion keyword
		{
			name: "portion_keyword",
			input: `
				package Test {
					part P {
						portion redefines x = 100;
					}
				}
			`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.input)))
			_ = p.ParseFile()
			
			if len(p.Diagnostics) > 0 {
				t.Logf("Diagnostics for %s:", tt.name)
				for _, d := range p.Diagnostics {
					t.Logf("  offset %d: %s", d.Span.Offset, d.Message)
				}
			} else {
				t.Logf("%s: CLEAN", tt.name)
			}
		})
	}
}
