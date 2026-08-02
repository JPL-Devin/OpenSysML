package libs

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask85AnonymousReturn(t *testing.T) {
	// Exact pattern from Performances.kerml with doc comment
	input := `package Test {
		abstract predicate BooleanEvaluation {
			doc
			/*
			 * Test doc comment.
			 */
		 
			return : Boolean[1];
		}
	}`
	
	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()
	
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(input) {
			char = string(input[d.Span.Offset])
		}
		t.Logf("  - offset=%d (char=%q): %s", d.Span.Offset, char, d.Message)
	}
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
