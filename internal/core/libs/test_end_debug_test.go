package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Debug: where does "end feature" fail?
func TestEndFeatureDebug(t *testing.T) {
	input := `
		connector SelfLink {
			end feature thisThing: Anything;
			end self2 [1] feature sameThing: Anything;
		}
	`

	p := parser.New(source.New("test.kerml", []byte(input)))
	root := p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - %s (offset %d)", d.Message, d.Span.Offset)
	}

	t.Logf("\n=== AST ===")
	if len(root.Members) > 0 {
		t.Logf("Root members: %d", len(root.Members))
	}
}
