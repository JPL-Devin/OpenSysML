package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask91ViewsPattern(t *testing.T) {
	input := `package Test {
		feature viewpointSatisfactions {
			ref :>> ownedPerformances::this, subperformances::this default that.that;
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
