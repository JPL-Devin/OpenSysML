package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseSuccessionNoMult(t *testing.T) {
	src := `
behavior def Test {
	succession body then untilDecision;
}
`
	sf := source.New("test.sysml", []byte(src))
	p := New(sf)
	f := p.ParseFile()
	
	// Debug: print tokens at offset 30-45
	t.Logf("Input: %q", src)
	t.Logf("Offset 30-45: %q", src[30:45])
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s [offset %d, len %d]", d.Message, d.Span.Offset, d.Span.Len)
		}
		t.Fatalf("Expected no errors, got %d", len(p.Diagnostics))
	}
	
	// Check parsed structure
	if len(f.Members) == 0 {
		t.Fatal("No declarations parsed")
	}
	
	t.Log("Succession without multiplicity parsed successfully!")
}
