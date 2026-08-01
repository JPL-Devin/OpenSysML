package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestAnonymousBoolWithBody(t *testing.T) {
	src := `
struct TimeSignal {
	private bool :>> signalCondition {
		doc
		/* comment */
		
		x.y == z
	}
}
`
	sf := source.New("test.sysml", []byte(src))
	p := New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("[offset %d] %s", d.Span.Offset, d.Message)
		}
	}
}
