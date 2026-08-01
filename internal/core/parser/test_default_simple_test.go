package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestDefaultSimple(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		attribute def Test {
			attribute x default 5;
		}
	`))
	p := New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
		}
		t.FailNow()
	}
	
	mem1 := root.Members[0].(*ast.Membership)
	def := mem1.Member.(*ast.Definition)
	mem2 := def.Members[0].(*ast.Membership)
	usage := mem2.Member.(*ast.Usage)
	
	if usage.Value == nil {
		t.Error("Expected value")
	}
	t.Logf("Success: default keyword works")
}
