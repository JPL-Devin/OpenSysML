package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestNumericalFunctions(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Function Library/NumericalFunctions.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("NumericalFunctions.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("Parse diagnostics (%d):", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Errorf("  %s [near: %q]", d.Message, text)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
