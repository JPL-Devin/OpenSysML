package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSingleFile_SampledFunctions_Details(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Analysis/SampledFunctions.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("SampledFunctions.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		if i >= 15 {
			t.Logf("  ... (%d more)", len(p.Diagnostics)-15)
			break
		}
		text := sf.Text(d.Span)
		if len(text) > 50 {
			text = text[:50] + "..."
		}
		t.Logf("  %s [near: %q]", d.Message, text)
	}
}
