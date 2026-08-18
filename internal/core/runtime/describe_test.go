package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestDescribeOperandEnumerationLiteral: an operator diagnostic names the
// literal an operand is, so the reader sees which enumeration it came from.
func TestDescribeOperandEnumerationLiteral(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
package D {
    enum def Color { red; green; }
}
attribute test = D::Color::red;
`)
	ctx := NewContext(model, resolver, 1000)
	sym := resolveSymbol(t, root, "test")
	val, err := ctx.Eval(sym.Decl.(*ast.Usage).Value)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := describeOperand(val); got != "the enumeration literal Color::red" {
		t.Errorf("describeOperand = %q", got)
	}
	// A literal that was never resolved is still described, never printed blank.
	if got := describeOperand(Value{Kind: ValEnumLiteral}); got != "the enumeration literal <unknown enumeration literal>" {
		t.Errorf("describeOperand of an unresolved literal = %q", got)
	}
}
