package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestOccurrenceFunctionsDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Function Library/OccurrenceFunctions.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("OccurrenceFunctions.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 40 {
				text = text[:40] + "..."
			}
			t.Logf("  [%d] offset=%d: %s [near: %q]", i+1, d.Span.Offset, d.Message, text)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
