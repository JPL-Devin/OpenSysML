package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestControlFunctionsDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Function Library/ControlFunctions.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("ControlFunctions.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		// First 10 errors
		for i, d := range p.Diagnostics {
			if i >= 10 {
				break
			}
			text := sf.Text(d.Span)
			if len(text) > 40 {
				text = text[:40] + "..."
			}
			t.Logf("  [%d] offset=%d: %s [near: %q]", i+1, d.Span.Offset, d.Message, text)
		}
		if len(p.Diagnostics) > 10 {
			t.Logf("  ... and %d more", len(p.Diagnostics)-10)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
