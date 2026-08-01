package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseArrowInvokeWithCoalesce(t *testing.T) {
	code := `package Test {
    datatype Array {
        feature flattenedSize: Integer[1] = dimensions->reduce '*' ?? 1;
    }
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
