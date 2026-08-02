package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestActionBodyStatementsDebug(t *testing.T) {
	input := `package Test {
		action Init {
			assign x := 1;
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()
	
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  offset=%d: %s", d.Span.Offset, d.Message)
		// Show character at offset
		if d.Span.Offset < len(input) {
			t.Logf("    character: %q", input[d.Span.Offset])
			// Show context
			start := d.Span.Offset - 30
			if start < 0 {
				start = 0
			}
			end := d.Span.Offset + 30
			if end > len(input) {
				end = len(input)
			}
			t.Logf("    context: ...%s", input[start:end])
		}
	}
}
