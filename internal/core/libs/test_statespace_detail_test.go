package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStateSpaceDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Analysis/StateSpaceRepresentation.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("StateSpaceRepresentation.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Logf("%s at offset %d [near: %q]", d.Message, d.Span.Offset, text)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
