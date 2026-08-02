package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestRemainingErrorPatterns(t *testing.T) {
	// Analyze all 13 remaining files
	files := []string{
		"Kernel Libraries/Kernel Semantic Library/Base.kerml",
		"Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml",
		"Kernel Libraries/Kernel Semantic Library/Flows.sysml",
		"Kernel Libraries/Kernel Semantic Library/Occurrences.kerml",
		"Systems Library/Actions.sysml",
		"Systems Library/Items.sysml",
		"Systems Library/States.sysml",
		"Systems Library/Views.sysml",
		"Domain Libraries/Analysis/TradeStudies.sysml",
		"Domain Libraries/Cause and Effect/CausationConnections.sysml",
		"Domain Libraries/Geometry/ShapeItems.sysml",
		"Domain Libraries/Geometry/Shapes.sysml",
		"Domain Libraries/Quantities and Units/ISQBase.sysml",
	}

	src := &embedSource{}

	for _, file := range files {
		data, err := src.Read(file)
		if err != nil {
			t.Logf("  %s: SKIP (load error)", file)
			continue
		}

		p := parser.New(source.New(file, data))
		_ = p.ParseFile()

		if len(p.Diagnostics) == 0 {
			continue
		}

		t.Logf("\n=== %s (%d errors) ===", file, len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			if i >= 3 {
				t.Logf("  ... and %d more", len(p.Diagnostics)-3)
				break
			}
			offset := d.Span.Offset
			context := ""
			if offset >= 10 && offset+30 < len(data) {
				context = string(data[offset-10 : offset+30])
			}
			t.Logf("  [%d] offset=%d: %s", i+1, offset, d.Message)
			t.Logf("      context: %q", context)
		}
	}
}
