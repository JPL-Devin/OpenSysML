package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStatePerformancesDetailTask67(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml")
	if err != nil {
		t.Fatalf("Failed to read StatePerformances.kerml: %v", err)
	}

	p := parser.New(source.New("StatePerformances.kerml", data))
	_ = p.ParseFile()

	t.Logf("Total diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		t.Logf("  [%d] Offset %d: %s", i, d.Span.Offset, d.Message)
	}
}
