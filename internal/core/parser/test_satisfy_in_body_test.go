package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSatisfyInViewBody(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		view def MyView {
			satisfy requirement viewpointConformance by that {
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
		t.FailNow()
	}
	
	mem1 := root.Members[0].(*ast.Membership)
	def := mem1.Member.(*ast.Definition)
	mem2 := def.Members[0].(*ast.Membership)
	usage := mem2.Member.(*ast.Usage)
	
	if usage.Kind != ast.UsageSatisfy {
		t.Errorf("Expected UsageSatisfy, got %v", usage.Kind)
	}
	t.Logf("Success!")
}
