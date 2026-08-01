package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseFunctionCallUnrestrictedName(t *testing.T) {
	code := `package Test {
    feature x = reduce('*');
}`
	sf := source.New("test.kerml", []byte(code))
	p := New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 60 {
				text = text[:60] + "..."
			}
			t.Logf("  %s [near: %q]", d.Message, text)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
