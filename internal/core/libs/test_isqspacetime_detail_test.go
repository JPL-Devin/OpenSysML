package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSingleFile_ISQSpaceTime_Details(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Quantities and Units/ISQSpaceTime.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("ISQSpaceTime.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Logf("  [%d] offset %d: %s [near: %q]", i+1, d.Span.Offset, d.Message, text)
		}
		
		// Show file size and check offsets
		data, _ := src.Read("Domain Libraries/Quantities and Units/ISQSpaceTime.sysml")
		t.Logf("File size: %d bytes", len(data))
	} else {
		t.Log("Parsed cleanly!")
	}
}
