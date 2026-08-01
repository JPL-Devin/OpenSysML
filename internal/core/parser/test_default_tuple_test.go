package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestDefaultTuple(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		attribute def Test {
			attribute :>> mRefs default (SI::m, SI::m);
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
		t.Errorf("Expected default value, got nil")
	}
}
