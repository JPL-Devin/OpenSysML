package parser_test
import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
func TestBodyExprInParam(t *testing.T) {
	code := []byte(`
calc def Test {
    attribute x = { in i; i > 0 };
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
	
	if root == nil {
		t.Fatal("root is nil")
	}
}
