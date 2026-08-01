package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSingleFile_Requirements_Details(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Requirements.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("Requirements.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			offset := d.Span.Offset
			end := offset + 80
			if end > len(data) {
				end = len(data)
			}
			context := data[offset:end]
			t.Logf("  [%d] offset %d: %s [context: %q]", i+1, offset, d.Message, context)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
