package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestDebugAnonDefault(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		attribute def Test {
			attribute :>> mRefs default (a, b, c) {
				doc /* comment */
			}
		}
	`))
	p := New(src)
	root := p.ParseFile()
	
	// Just show all diagnostics with detailed context
	for i, d := range p.Diagnostics {
		text := src.Text(d.Span)
		before := ""
		if d.Span.Offset > 10 {
			beforeSpan := source.Span{Offset: d.Span.Offset - 10, Len: 10}
			before = src.Text(beforeSpan)
		}
		t.Logf("[%d] offset %d: %s", i+1, d.Span.Offset, d.Message)
		t.Logf("     Context: ...%s[%s]...", before, text)
	}
	
	if root == nil {
		t.Fatal("Root is nil")
	}
	
	t.Logf("Parsed with %d errors", len(p.Diagnostics))
}
