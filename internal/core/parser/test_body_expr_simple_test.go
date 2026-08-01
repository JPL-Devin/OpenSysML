package parser_test
import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
func TestBodyExprAsRHS(t *testing.T) {
	code := []byte(`
attribute def Test {
    attribute x = { in i; i > 0 };
}
`)
	src := source.New("test.sysml", code)
	p := parser.New(src)
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("error: %s", d.Message)
		}
		t.Fatal("parse failed")
	}
	
	def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
	usage := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	
	if usage.Value == nil {
		t.Fatal("expected value expression")
	}
	
	bodyExpr, ok := usage.Value.(*ast.BodyExpr)
	if !ok {
		t.Fatalf("expected BodyExpr, got %T", usage.Value)
	}
	
	if len(bodyExpr.Params) == 0 {
		t.Fatal("expected body parameters")
	}
	
	t.Logf("Body expression parsed: %d params", len(bodyExpr.Params))
}
