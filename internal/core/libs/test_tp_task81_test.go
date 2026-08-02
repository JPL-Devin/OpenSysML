package libs

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTPTask81Detail(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}
	
	p := parser.New(source.New("TransitionPerformances.kerml", content))
	_ = p.ParseFile()
	
	t.Logf("TransitionPerformances diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(content) {
			char = string(content[d.Span.Offset])
		}
		t.Logf("%d. offset=%d (char=%q): %s", i+1, d.Span.Offset, char, d.Message)
	}
}
