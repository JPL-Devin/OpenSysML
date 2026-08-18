package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

func TestFRPContextTask81(t *testing.T) {
	src := &embedSource{}
	content, _ := src.Read("Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml")

	offsets := []int{6085, 6092, 7680}
	for _, offset := range offsets {
		start := offset - 50
		if start < 0 {
			start = 0
		}
		end := offset + 50
		if end > len(content) {
			end = len(content)
		}

		char := ""
		if offset < len(content) {
			char = string(content[offset])
		}

		t.Logf("\n=== Offset %d (char=%q) ===", offset, char)
		t.Logf("Context: %q", string(content[start:end]))
	}

	// Also run parser to get exact error messages
	p := parser.New(source.New("test.kerml", content))
	_ = p.ParseFile()

	t.Logf("\n=== Diagnostics ===")
	for _, d := range p.Diagnostics {
		t.Logf("offset=%d: %s", d.Span.Offset, d.Message)
	}
}
