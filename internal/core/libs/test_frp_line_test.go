package libs

import (
	"strings"
	"testing"
)

func TestFRPLine(t *testing.T) {
	src := &embedSource{}
	content, _ := src.Read("Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml")

	// Find line containing offset 6085
	offset := 6085
	start := 0
	lineNum := 1
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			lineNum++
			start = i + 1
		}
	}

	// Extract full line
	end := start
	for end < len(content) && content[end] != '\n' && content[end] != '\r' {
		end++
	}

	line := string(content[start:end])
	t.Logf("Line %d: %q", lineNum, strings.TrimSpace(line))

	// Find surrounding context (3 lines before, 3 lines after)
	lines := strings.Split(string(content), "\n")
	contextStart := lineNum - 4
	if contextStart < 0 {
		contextStart = 0
	}
	contextEnd := lineNum + 3
	if contextEnd > len(lines) {
		contextEnd = len(lines)
	}

	t.Logf("\n=== Context (lines %d-%d) ===", contextStart+1, contextEnd)
	for i := contextStart; i < contextEnd && i < len(lines); i++ {
		marker := " "
		if i == lineNum-1 {
			marker = ">"
		}
		t.Logf("%s %4d: %s", marker, i+1, lines[i])
	}
}
