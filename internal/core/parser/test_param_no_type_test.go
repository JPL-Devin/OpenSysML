package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseParamNoType(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		calc def Test {
			in seq[1..*] nonunique ordered;
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
	
	if usage.Ident.Name != "seq" {
		t.Errorf("Expected name 'seq', got %q", usage.Ident.Name)
	}
	if usage.Direction != ast.DirIn {
		t.Errorf("Expected direction In, got %v", usage.Direction)
	}
	if usage.Multiplicity == nil {
		t.Errorf("Expected multiplicity, got nil")
	}
	if !usage.IsOrdered || !usage.IsNonunique {
		t.Errorf("Expected ordered nonunique, got ordered=%v nonunique=%v", usage.IsOrdered, usage.IsNonunique)
	}
}
