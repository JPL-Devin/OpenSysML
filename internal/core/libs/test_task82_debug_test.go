package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

func TestTask82Debug(t *testing.T) {
	// Simpler pattern first
	input := `package Test {
		assoc HappensWhile {
			end happensWhile [1] feature thatOccurrence: Occurrence;
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Test 1 (no relationship) - Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}

	// Now with relationship
	input2 := `package Test {
		assoc HappensWhile {
			end happensWhile [1] subsets x feature thatOccurrence: Occurrence;
		}
	}`

	p2 := parser.New(source.New("test2.kerml", []byte(input2)))
	_ = p2.ParseFile()

	t.Logf("\nTest 2 (with subsets) - Diagnostics: %d", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}
}
