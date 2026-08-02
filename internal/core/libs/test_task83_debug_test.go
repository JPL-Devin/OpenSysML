package libs

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask83ReturnDebug(t *testing.T) {
	input := `package Test {
		predicate IncomingTransferSort {
			return t1First: Boolean [1];
		}
	}`
	
	p := parser.New(source.New("test.kerml", []byte(input)))
	root := p.ParseFile()
	
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(input) {
			char = string(input[d.Span.Offset])
		}
		t.Logf("  - offset=%d (char=%q): %s", d.Span.Offset, char, d.Message)
	}
	
	// Also dump AST to see what was parsed
	if root != nil {
		t.Logf("Root has %d members", len(root.Members))
	}
}
