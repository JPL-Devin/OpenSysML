package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask79Require(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "require in requirement body",
			input: `package Test {
				requirement r {
					require viewpointSatisfactions {
						doc /* constraint */
					}
				}
			}`,
		},
		{
			name: "require in satisfy body",
			input: `package Test {
				satisfy requirement viewpointConformance by that {
					require viewpointSatisfactions {
						doc /* constraint */
					}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d diagnostics:", len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
				}
			}
		})
	}
}
