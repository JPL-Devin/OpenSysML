package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestValueWithBody(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		attribute def Test {
			attribute :>> mRefs default (a, b, c) {
				doc /* comment */
			}
		}
	`))
	p := New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("offset %d: %s", d.Span.Offset, d.Message)
		}
		t.Fatalf("Expected no errors")
	}
	
	mem1 := root.Members[0].(*ast.Membership)
	def := mem1.Member.(*ast.Definition)
	mem2 := def.Members[0].(*ast.Membership)
	usage := mem2.Member.(*ast.Usage)
	
	if usage.Value == nil {
		t.Error("Expected value expression")
	}
	if !usage.HasBody {
		t.Error("Expected body")
	}
	if len(usage.Members) == 0 {
		t.Error("Expected body members (doc)")
	}
	t.Logf("Success: value + body")
}
