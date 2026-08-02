package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask91Minimal(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "ref in feature body",
			input: `package Test {
				feature x {
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}`,
		},
		{
			name: "ref in constraint body",
			input: `package Test {
				constraint x {
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}`,
		},
		{
			name: "ref in require body",
			input: `package Test {
				require x {
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.input)))
			_ = p.ParseFile()

			t.Logf("Diagnostics: %d", len(p.Diagnostics))
			for _, d := range p.Diagnostics {
				t.Logf("  - %s", d.Message)
			}

			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
			}
		})
	}
}
