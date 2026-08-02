package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestOccurrencesTask80(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	p := parser.New(source.New("Occurrences.kerml", data))
	_ = p.ParseFile()
	
	t.Logf("Occurrences.kerml: %d diagnostics", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}
		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
	}
}
