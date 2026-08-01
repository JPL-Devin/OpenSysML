package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestInvSimple(t *testing.T) {
	input := `datatype Test { inv { x == 0 } }`
	
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			t.Logf("  offset=%d: %s [near: %q]", d.Span.Offset, d.Message, text)
		}
		t.Fatalf("Expected clean parse")
	}
}

func TestInvFunctionCall(t *testing.T) {
	input := `datatype Test { inv { isZero(zero) } }`
	
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			t.Logf("  offset=%d: %s [near: %q]", d.Span.Offset, d.Message, text)
		}
		t.Fatalf("Expected clean parse")
	}
}
