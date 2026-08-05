package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask80ReturnInBody(t *testing.T) {
	// Simplest test - return statement inside feature body
	input := `package Test {
		function test {
			return result = 42;
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
