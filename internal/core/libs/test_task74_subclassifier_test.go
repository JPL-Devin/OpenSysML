package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask74Subclassifier(t *testing.T) {
	input := `package Test {
		subclassifier SelfLink specializes Base {
			doc
			/* SelfLink is a subtype of Base. */
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - %s", d.Message)
		}
	}
}
