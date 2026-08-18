package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask70BodyExprMultiline(t *testing.T) {
	// Simplified version of StatePerformances.kerml line 77-83
	input := `package Test {
		function allSubstatePerformances {
			in p : Performance;
			return result: StatePerformance[*];
		}
		function includes {
			in seq: Integer[*];
			in elem: Integer;
			return result: Boolean;
		}
		function isEmpty {
			in seq: Integer[*];
			return result: Boolean;
		}
		
		classifier StatePerformance {
			feature accepted: Integer[0..1];
			feature accableT: Integer;
			feature dispatchScope: Performance;
			feature thatSP: StatePerformance;
			feature exit: StatePerformance;
			
			inv { allSubstatePerformances(dispatchScope)->forAll{in oSP : StatePerformance;						
					  oSP == thatSP | isEmpty(oSP.accepted) |
   					  includes(thatSP.exit.successors, oSP.exit) |
					  ( oSP.accepted != accableT & true ) } }
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	t.Logf("Input length: %d bytes", len(input))
	t.Logf("Char at 668: %q", string(input[668:669]))
	t.Logf("Context 660-680: %q", string(input[660:680]))
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
