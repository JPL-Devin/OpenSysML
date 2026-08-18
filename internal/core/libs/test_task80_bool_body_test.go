package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask80ReturnInBoolBody(t *testing.T) {
	// Bool usage with return statement inside body
	input := `package Test {
		bool test {
			return result = true;
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
