package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"testing"
)

func TestTask81SuccessionWithIdentMult(t *testing.T) {
	input := `package Test {
		feature taNum: Natural [1] = 5;
		
		succession triggerAfter [taNum] first [0..1] a then [*] b;
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
