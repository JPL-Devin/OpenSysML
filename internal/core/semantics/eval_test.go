package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// evalExpr parses src as a standalone expression and evaluates it.
func evalExpr(t *testing.T, src string) (Value, bool) {
	t.Helper()
	p := parser.New(source.New("<e>", []byte(src)))
	expr := p.ParseExpression()
	if expr == nil {
		t.Fatalf("failed to parse expression %q", src)
	}
	m := NewModel(nil)
	return m.Eval(expr)
}

func TestEvalIntLiteralsAndArithmetic(t *testing.T) {
	cases := map[string]int64{
		"42":          42,
		"1 + 2":       3,
		"2 * 3 + 4":   10,
		"10 - 3":      7,
		"10 % 3":      1,
		"-5":          -5,
		"2 * (3 + 4)": 14,
	}
	for src, want := range cases {
		v, ok := evalExpr(t, src)
		if !ok || v.Kind != ValInt || v.Int != want {
			t.Fatalf("%q = %+v ok=%v, want int %d", src, v, ok, want)
		}
	}
}

func TestEvalRealArithmetic(t *testing.T) {
	v, ok := evalExpr(t, "10 / 4")
	if !ok || v.Kind != ValReal || v.Real != 2.5 {
		t.Fatalf("10/4 = %+v ok=%v, want real 2.5", v, ok)
	}
}

// A quotient of operands beyond 2^53 folds as the exact ratio rounded once,
// the same answer the runtime computes.
func TestEvalFoldsExactQuotientBeyondFloatRange(t *testing.T) {
	v, ok := evalExpr(t, "9007199254740993 / 3")
	if !ok || v.Kind != ValReal || v.Real != 3002399751580331 {
		t.Fatalf("9007199254740993/3 = %+v ok=%v, want real 3002399751580331", v, ok)
	}
}

func TestEvalBooleanLogic(t *testing.T) {
	cases := map[string]bool{
		"true and false": false,
		"true or false":  true,
		"not true":       false,
		"1 < 2":          true,
		"3 <= 3":         true,
		"2 == 2":         true,
		"2 != 3":         true,
		"5 > 10":         false,
	}
	for src, want := range cases {
		v, ok := evalExpr(t, src)
		if !ok || v.Kind != ValBool || v.Bool != want {
			t.Fatalf("%q = %+v ok=%v, want bool %v", src, v, ok, want)
		}
	}
}

func TestEvalConditional(t *testing.T) {
	v, ok := evalExpr(t, "if 1 < 2 ? 10 else 20")
	if !ok || v.Kind != ValInt || v.Int != 10 {
		t.Fatalf("conditional = %+v ok=%v, want 10", v, ok)
	}
}

func TestEvalInfinityLiteral(t *testing.T) {
	m := NewModel(nil)
	v, ok := m.Eval(&ast.LiteralInfinity{})
	if !ok || v.Kind != ValInfinity {
		t.Fatalf("infinity = %+v ok=%v", v, ok)
	}
}

func TestEvalNonEvaluableReference(t *testing.T) {
	// A feature reference is not a model-level constant.
	if _, ok := evalExpr(t, "x + 1"); ok {
		t.Fatalf("expected feature reference to be non-evaluable")
	}
}

func TestEvalDivByZeroNotEvaluable(t *testing.T) {
	if _, ok := evalExpr(t, "1 / 0"); ok {
		t.Fatalf("division by zero should be non-evaluable")
	}
}
