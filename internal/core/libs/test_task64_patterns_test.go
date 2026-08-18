package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

func TestTask64FindTopPatterns(t *testing.T) {
	src := &embedSource{}

	files := []string{
		"Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml",
		"Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml",
		"Domain Libraries/Analysis/TradeStudies.sysml",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			data, err := src.Read(name)
			if err != nil {
				t.Fatalf("Failed to load %s: %v", name, err)
			}

			file := source.New(name, data)
			p := parser.New(file)
			root := p.ParseFile()

			t.Logf("\n%s diagnostics:", name)
			diags := p.Diagnostics
			for i, d := range diags {
				if i >= 10 {
					break
				}
				t.Logf("  [%d] offset %d: %s", i, d.Span.Offset, d.Message)
				// Show context
				start := d.Span.Offset
				if start < 0 {
					start = 0
				}
				end := start + 80
				if end > len(data) {
					end = len(data)
				}
				t.Logf("      context: %q", data[start:end])
			}

			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
		})
	}
}
