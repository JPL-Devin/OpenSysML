package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

func TestMultiplicityDeclaration(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "exactlyOne",
			input: `package Test {
				multiplicity exactlyOne [1..1] {
					doc /* exactly one */
				}
			}`,
		},
		{
			name: "zeroOrOne",
			input: `package Test {
				multiplicity zeroOrOne [0..1] {
					doc /* zero or one */
				}
			}`,
		},
		{
			name: "no body",
			input: `package Test {
				multiplicity custom [2..5];
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) > 0 {
				for _, d := range p.Diagnostics {
					t.Errorf("  %s", d.Message)
				}
			}
		})
	}
}
