package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"testing"
)

func TestFRPTask81Detail(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("FeatureReferencingPerformances.kerml", content))
	_ = p.ParseFile()

	t.Logf("FRP diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}
}
