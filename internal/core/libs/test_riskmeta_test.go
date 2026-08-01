package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestRiskMetadata(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Metadata/RiskMetadata.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("RiskMetadata.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		text := sf.Text(d.Span)
		if len(text) > 50 {
			text = text[:50] + "..."
		}
		t.Logf("  [%d] offset %d: %s [near: %q]", i, d.Span.Offset, d.Message, text)
	}
}
