package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseKeywordAsName(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		part def Item {
			part done: Item;
		}
	`))
	p := New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("%s", d.Message)
		}
		t.FailNow()
	}

	// root -> membership -> definition -> membership -> usage
	mem1 := root.Members[0].(*ast.Membership)
	def := mem1.Member.(*ast.Definition)
	mem2 := def.Members[0].(*ast.Membership)
	usage := mem2.Member.(*ast.Usage)

	if usage.Ident.Name != "done" {
		t.Errorf("Expected usage name 'done', got %q", usage.Ident.Name)
	}
}
