package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestAnonymousAttributeValue(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		attribute def Test {
			attribute :>> mRefs = (a, b, c);
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
	
	if usage.Ident.Name != "" {
		t.Errorf("Expected anonymous (empty name), got %q", usage.Ident.Name)
	}
	if len(usage.Relationships) == 0 {
		t.Error("Expected redefines relationship")
	}
	if usage.Value == nil {
		t.Error("Expected value expression")
	}
	t.Logf("Success: anonymous attribute with redefines + value")
}
