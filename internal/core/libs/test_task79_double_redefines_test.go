package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask79DoubleRedefines(t *testing.T) {
	input := `package Test {
		feature f {
			private ref redefines Item::incomingTransferSort, subobjects::incomingTransferSort;
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d diagnostics:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
		}
	}
}
