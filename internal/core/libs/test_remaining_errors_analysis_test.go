package libs

import (
	"strings"
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestRemainingErrorsAnalysis(t *testing.T) {
	src := &embedSource{}
	files := []string{
		"Systems Library/Actions.sysml",
		"Domain Libraries/Geometry/ShapeItems.sysml",
		"Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml",
		"Kernel Libraries/Kernel Semantic Library/Performances.kerml",
		"Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml",
		"Kernel Libraries/Kernel Semantic Library/Flows.sysml",
		"Systems Library/Views.sysml",
		"Analysis Library/TradeStudies.sysml",
	}
	
	for _, filename := range files {
		content, err := src.Read(filename)
		if err != nil {
			t.Logf("Skip %s: %v", filename, err)
			continue
		}
		
		p := parser.New(source.New(filename, content))
		_ = p.ParseFile()
		
		if len(p.Diagnostics) == 0 {
			continue
		}
		
		t.Logf("\n=== %s (%d errors) ===", filename, len(p.Diagnostics))
		
		// Group by offset to dedupe cascading errors
		offsetsSeen := make(map[int]bool)
		
		for _, d := range p.Diagnostics {
			if offsetsSeen[d.Span.Offset] {
				continue
			}
			offsetsSeen[d.Span.Offset] = true
			
			// Get line number
			lineNum := 1
			for i := 0; i < d.Span.Offset && i < len(content); i++ {
				if content[i] == '\n' {
					lineNum++
				}
			}
			
			// Get context
			lines := strings.Split(string(content), "\n")
			if lineNum > 0 && lineNum <= len(lines) {
				line := lines[lineNum-1]
				if len(line) > 100 {
					line = line[:100] + "..."
				}
				char := ""
				if d.Span.Offset < len(content) {
					char = string(content[d.Span.Offset])
				}
				t.Logf("Line %d (offset %d, char=%q): %s", lineNum, d.Span.Offset, char, d.Message)
				t.Logf("  Context: %s", line)
			}
		}
	}
}
