package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestVerificationCasesDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/VerificationCases.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("VerificationCases.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Logf("  %s at %q (offset %d)", d.Message, text, d.Span.Offset)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
