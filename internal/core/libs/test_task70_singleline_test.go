package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask70BodyExprSingleLine(t *testing.T) {
	// Single-line version - no newlines
	input := `package Test {
		function allSubstatePerformances { in p : Performance; return result: StatePerformance[*]; }
		function includes { in seq: Integer[*]; in elem: Integer; return result: Boolean; }
		function isEmpty { in seq: Integer[*]; return result: Boolean; }
		
		classifier StatePerformance {
			feature accepted: Integer[0..1];
			feature accableT: Integer;
			feature dispatchScope: Performance;
			feature thatSP: StatePerformance;
			feature exit: StatePerformance;
			
			inv { allSubstatePerformances(dispatchScope)->forAll{in oSP : StatePerformance; oSP == thatSP | isEmpty(oSP.accepted) | includes(thatSP.exit, oSP.exit) | ( oSP.accepted != accableT & true ) } }
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
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
