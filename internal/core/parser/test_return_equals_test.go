package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseReturnEquals(t *testing.T) {
	src := `
calc def Test {
	return sampling = new Thing();
}
`
	sf := source.New("test.sysml", []byte(src))
	p := New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s [offset %d]", d.Message, d.Span.Offset)
		}
		t.Fatalf("Expected no errors, got %d", len(p.Diagnostics))
	}
	
	t.Log("Return with = expr parsed successfully!")
}
