package libs

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestOccurrencesContext(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	p := parser.New(source.New("Occurrences.kerml", data))
	_ = p.ParseFile()
	
	// Get context for first 3 unique offset errors
	offsets := []int{25224, 25336, 28079}
	
	for _, offset := range offsets {
		start := offset - 80
		if start < 0 {
			start = 0
		}
		end := offset + 80
		if end > len(data) {
			end = len(data)
		}
		
		context := string(data[start:end])
		char := ""
		if offset < len(data) {
			char = string(data[offset])
		}
		
		t.Logf("\nOffset %d (char=%q):", offset, char)
		t.Logf("Context: %q", context)
		
		// Find line number
		lineNum := 1
		for i := 0; i < offset && i < len(data); i++ {
			if data[i] == '\n' {
				lineNum++
			}
		}
		t.Logf("Line ~%d", lineNum)
		fmt.Println()
	}
}
