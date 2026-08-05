package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask80OccurrencesMinimal(t *testing.T) {
	// Exact pattern from Occurrences.kerml line 702-706
	input := `package Test {
		function includes {
			in seq: Integer[*];
			in elem: Integer;
			return result: Boolean;
		}
		
		struct Transfer {
			feature endShot: Integer;
		}
		
		struct IncomingTransferSort {
			in t1: Transfer [1];
			in t2: Transfer [1];
			return t1First: Boolean [1];
		}
		
		bool earlierFirstIncomingTransferSort : IncomingTransferSort {
			return t1First = includes(t1.endShot.successors, t2.endShot);
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Diagnostics: %d", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			byteOffset := d.Span.Offset
			char := ""
			if byteOffset < len(input) {
				char = string(input[byteOffset])
			}
			t.Logf("  offset=%d (char=%q): %s", byteOffset, char, d.Message)
		}
		t.Errorf("Expected clean parse")
	}
}
