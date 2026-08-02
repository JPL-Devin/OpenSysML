package libs

import (
	"strings"
	"testing"
)

func TestOccurrencesContext(t *testing.T) {
	src := &embedSource{}
	content, _ := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")
	
	offsets := []int{25224, 28079, 30619, 31030}
	for _, offset := range offsets {
		lineNum := 1
		for i := 0; i < offset && i < len(content); i++ {
			if content[i] == '\n' {
				lineNum++
			}
		}
		
		lines := strings.Split(string(content), "\n")
		contextStart := lineNum - 3
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := lineNum + 3
		if contextEnd > len(lines) {
			contextEnd = len(lines)
		}
		
		char := ""
		if offset < len(content) {
			char = string(content[offset])
		}
		
		t.Logf("\n=== Offset %d (char=%q, line %d) ===", offset, char, lineNum)
		for i := contextStart; i < contextEnd && i < len(lines); i++ {
			marker := " "
			if i == lineNum-1 {
				marker = ">"
			}
			t.Logf("%s %4d: %s", marker, i+1, lines[i])
		}
	}
}
