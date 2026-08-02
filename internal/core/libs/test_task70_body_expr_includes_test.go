package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask70BodyExprIncludes(t *testing.T) {
	input := `package Test {
		function includes {
			in seq: Integer[*];
			in elem: Integer;
			return result: Boolean;
		}
		
		function test {
			return result: Boolean = 
				[1,2,3]->collect{in oSP : Integer;
					oSP == 1 | includes([1,2], oSP) };
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()
	
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
