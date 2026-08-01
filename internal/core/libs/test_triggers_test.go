package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTriggersDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Triggers.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("Triggers.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 80 {
				text = text[:80] + "..."
			}
			t.Logf("  [offset %d] %s [near: %q]", d.Span.Offset, d.Message, text)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
