package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask91ViewsExact(t *testing.T) {
	// Exact structure from Views.sysml lines 40-50
	input := `package Views {
		view def View {
			satisfy requirement viewpointConformance by that {
				require viewpointSatisfactions {
					doc
					/*
					 * The required ViewpointChecks.
					 */
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(input) {
			char = string(input[byteOffset])
		}
		t.Logf("  - offset=%d (char=%q): %s", byteOffset, char, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
