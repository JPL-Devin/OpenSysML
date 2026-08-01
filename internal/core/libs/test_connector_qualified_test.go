package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestConnectorQualifiedName(t *testing.T) {
	code := `
		calc def Thing {
			attribute occ: Anything[1];
			private connector : Thing from occ.startShot to self;
		}
	`
	
	sf := source.New("test.sysml", []byte(code))
	p := parser.New(sf)
	tree := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 40 {
				text = text[:40] + "..."
			}
			t.Logf("Error: %s at %q (offset %d)", d.Message, text, d.Span.Offset)
		}
		t.Fatalf("Expected 0 errors, got %d", len(p.Diagnostics))
	}
	
	if tree == nil {
		t.Fatal("ParseFile returned nil")
	}
}
