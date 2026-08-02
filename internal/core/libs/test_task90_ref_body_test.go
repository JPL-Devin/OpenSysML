package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestRefWithBody(t *testing.T) {
	input := `package Test {
		flow def Message {
			ref payload [0..*] {
				doc
				/* payload */
			}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			byteOffset := d.Span.Offset
			char := ""
			inputBytes := []byte(input)
			if byteOffset < len(inputBytes) {
				char = string(inputBytes[byteOffset])
			}
			t.Logf("  - offset=%d (char=%q): %s", byteOffset, char, d.Message)
		}
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
