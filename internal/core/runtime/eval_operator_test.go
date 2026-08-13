package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestUnimplementedOperatorReportsWhy requires an operator the runtime does not
// evaluate to say what it would need, rather than failing as "unsupported".
func TestUnimplementedOperatorReportsWhy(t *testing.T) {
	const src = `calc def classify { in n : Integer; return : Boolean = n hastype Integer; }`

	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)
	classify := resolveSymbol(t, root, "classify")

	_, err := ctx.InvokeCalc(classify, []Value{constInt(1)}, root)
	if !errors.Is(err, ErrUnsupportedOperator) {
		t.Fatalf("InvokeCalc: got %v, want ErrUnsupportedOperator", err)
	}
	if !strings.Contains(err.Error(), "runtime type") {
		t.Fatalf("InvokeCalc: %v does not say what classification would need", err)
	}
}
