package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestClassifierAtNamespaceLevel(t *testing.T) {
	code := `package Base {
	abstract classifier Anything {
	}
}`
	
	p := parser.New(source.New("test.kerml", []byte(code)))
	_ = p.ParseFile()
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		offset := d.Span.Offset
		t.Logf("  - offset=%d, %s", offset, d.Message)
		
		// Show char at offset
		if offset < len(code) {
			t.Logf("    Char at offset: %q", code[offset])
			t.Logf("    Context: %q", code[offset:offset+15])
		}
	}
}
