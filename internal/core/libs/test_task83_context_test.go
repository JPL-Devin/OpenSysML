package libs

import (
	"strings"
	"testing"
)

func TestTask83Context(t *testing.T) {
	src := &embedSource{}
	content, _ := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")

	offset := 25224
	lineNum := 1
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			lineNum++
		}
	}

	lines := strings.Split(string(content), "\n")
	contextStart := lineNum - 10
	if contextStart < 0 {
		contextStart = 0
	}
	contextEnd := lineNum + 5
	if contextEnd > len(lines) {
		contextEnd = len(lines)
	}

	char := ""
	if offset < len(content) {
		char = string(content[offset])
	}

	t.Logf("=== Offset %d (char=%q, line %d) ===", offset, char, lineNum)
	for i := contextStart; i < contextEnd && i < len(lines); i++ {
		marker := " "
		if i == lineNum-1 {
			marker = ">"
		}
		t.Logf("%s %4d: %s", marker, i+1, lines[i])
	}
}
