package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestOccurrenceFunctions(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Function Library/OccurrenceFunctions.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("OccurrenceFunctions.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	t.Logf("Total diagnostics: %d", len(p.Diagnostics))
	if len(p.Diagnostics) > 0 {
		for i, d := range p.Diagnostics {
			if i >= 10 { break }
			text := sf.Text(d.Span)
			if len(text) > 40 {
				text = text[:40] + "..."
			}
			t.Logf("[%d] %s at %q (offset %d)", i+1, d.Message, text, d.Span.Offset)
		}
	}
}
