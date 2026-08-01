package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestConnectorKeyword(t *testing.T) {
	code := `
		part def Thing {
			private connector all during: Thing[0..1] from self to other;
			attribute other: Thing[1];
		}
	`
	
	sf := source.New("test.sysml", []byte(code))
	p := parser.New(sf)
	tree := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			t.Logf("Error: %s at %q", d.Message, text)
		}
		t.Fatalf("Expected 0 errors, got %d", len(p.Diagnostics))
	}
	
	if tree == nil {
		t.Fatal("ParseFile returned nil")
	}
}
