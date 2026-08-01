package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSingleFile_ModelingMetadata(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Metadata/ModelingMetadata.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("ModelingMetadata.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Logf("  Line %d, offset %d: %s [near: %q]", 0, d.Span.Offset, d.Message, text)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
