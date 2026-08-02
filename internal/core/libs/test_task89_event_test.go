package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask89Event(t *testing.T) {
	input := `package Test {
		flow def Message {
			in event occurrence sourceEvent [1] default self.start {
				doc
				/* start */
			}
			in event occurrence targetEvent [1] default self.done {
				doc
				/* end */
			}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("  - %s", d.Message)
		}
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
