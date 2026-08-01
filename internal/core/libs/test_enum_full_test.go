package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestEnumFull(t *testing.T) {
	src := `
standard library package RiskMetadata {
	private import ScalarValues::Real;
	
	attribute def Level :> Real {
		assert constraint { that >= 0.0 and that <= 1.0 }
	}
	
	enum def LevelEnum :> Level {
		low = 0.25;
		medium = 0.50;
		high = 0.75;
	}
}
`
	sf := source.New("test.sysml", []byte(src))
	p := parser.New(sf)
	_ = p.ParseFile()
	
	t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		text := sf.Text(d.Span)
		if len(text) > 30 {
			text = text[:30] + "..."
		}
		t.Logf("  [%d] offset %d: %s [near: %q]", i, d.Span.Offset, d.Message, text)
	}
}
