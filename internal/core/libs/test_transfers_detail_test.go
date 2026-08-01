package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTransfersDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Transfers.kerml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("Transfers.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		if i >= 10 { break }
		text := sf.Text(d.Span)
		if len(text) > 30 {
			text = text[:30] + "..."
		}
		t.Logf("  [%d] offset=%d msg=%q near=%q", i, d.Span.Offset, d.Message, text)
	}
}
