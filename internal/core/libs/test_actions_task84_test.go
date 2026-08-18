package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

func TestActionsTask84Detail(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Systems Library/Actions.sysml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("Actions.sysml", content))
	_ = p.ParseFile()

	t.Logf("Actions.sysml diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(content) {
			char = string(content[d.Span.Offset])
		}
		t.Logf("%d. offset=%d (char=%q): %s", i+1, d.Span.Offset, char, d.Message)
	}
}
