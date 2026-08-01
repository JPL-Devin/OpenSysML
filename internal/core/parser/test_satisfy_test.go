package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSatisfy(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		package Test {
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
	pkg := mem1.Member.(*ast.Package)
	mem2 := pkg.Members[0].(*ast.Membership)
	usage := mem2.Member.(*ast.Usage)
	
	if usage.Kind != ast.UsageSatisfy {
		t.Errorf("Expected UsageSatisfy, got %v", usage.Kind)
	}
	t.Logf("Success: satisfy parsed")
}
