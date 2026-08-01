package parser_test
import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
func TestReturnBodyExpr(t *testing.T) {
	code := []byte(`
calc def Test {
    return x = { in i; i > 0 };
}
`)
	src := source.New("test.sysml", code)
	p := parser.New(src)
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("error: %s [near: %q]", d.Message, src.Text(d.Span))
		}
		t.Fatal("parse failed")
	}
	
	def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
	resultUsage := def.Members[0].(*ast.Usage)
	
	if resultUsage.Value == nil {
		t.Fatal("expected value expression")
	}
	
	bodyExpr, ok := resultUsage.Value.(*ast.BodyExpr)
	if !ok {
		t.Fatalf("expected BodyExpr, got %T", resultUsage.Value)
	}
	
	t.Logf("Return with body expression: %d params", len(bodyExpr.Params))
}
