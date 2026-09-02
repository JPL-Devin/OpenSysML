package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// parseExpr parses src as one expression, so a test can evaluate the same node twice.
func parseExpr(t *testing.T, src string) ast.Node {
	t.Helper()
	expr := parser.New(source.New("<e>", []byte(src))).ParseExpression()
	if expr == nil {
		t.Fatalf("failed to parse %q", src)
	}
	return expr
}

// TestRepeatedLiteralEvaluationAnswersAlike: a literal evaluated again in one
// context, memoized or not, answers the same value, and one outside its range
// reports the same error each time rather than a cached success.
func TestRepeatedLiteralEvaluationAnswersAlike(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(model, resolver, 1000)

	for _, tc := range []struct {
		src  string
		want semantics.Value
	}{
		{"42", semantics.Value{Kind: semantics.ValInt, Int: 42}},
		{"2.5", semantics.Value{Kind: semantics.ValReal, Real: 2.5}},
	} {
		expr := parseExpr(t, tc.src)
		for i := 0; i < 3; i++ {
			got, err := ctx.Eval(expr)
			if err != nil || got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("eval %d of %s = %+v, %v; want %+v", i, tc.src, got, err, tc.want)
			}
		}
	}

	for _, src := range []string{"9223372036854775808", "1e400"} {
		expr := parseExpr(t, src)
		var first string
		for i := 0; i < 3; i++ {
			got, err := ctx.Eval(expr)
			if !errors.Is(err, semantics.ErrArithmeticOverflow) {
				t.Fatalf("eval %d of %s = %+v, %v; want ErrArithmeticOverflow", i, src, got, err)
			}
			if i == 0 {
				first = err.Error()
			} else if err.Error() != first {
				t.Fatalf("eval %d of %s reported %q; first reported %q", i, src, err.Error(), first)
			}
		}
	}
}

// TestRepeatedInvocationEvaluationAnswersAlike: an invocation expression
// evaluated again in one context reaches the same calc, and one naming no
// declaration is refused the same way each time, after its arguments.
func TestRepeatedInvocationEvaluationAnswersAlike(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		calc def twice { in x : Integer; return : Integer = x + x; }
		calc def fib { in k : Integer; return : Integer = if k <= 1 ? k else fib(k - 1) + fib(k - 2); }
	`)
	ctx := NewContext(model, resolver, 100000)
	ec := NewEvalContext(ctx, root)

	for _, tc := range []struct {
		src  string
		want int64
	}{
		{"twice(21)", 42},
		{"fib(10)", 55},
	} {
		expr := parseExpr(t, tc.src)
		for i := 0; i < 3; i++ {
			got, err := ec.Eval(expr)
			if err != nil || got.Kind != ValConst || got.Const.Int != tc.want {
				t.Fatalf("eval %d of %s = %+v, %v; want %d", i, tc.src, got, err, tc.want)
			}
		}
	}

	missing := parseExpr(t, "nowhere(1)")
	var first string
	for i := 0; i < 3; i++ {
		got, err := ec.Eval(missing)
		if !errors.Is(err, ErrUnresolvedReference) {
			t.Fatalf("eval %d of nowhere(1) = %+v, %v; want ErrUnresolvedReference", i, got, err)
		}
		if i == 0 {
			first = err.Error()
		} else if err.Error() != first {
			t.Fatalf("eval %d reported %q; first reported %q", i, err.Error(), first)
		}
	}

	// An argument that fails is reported before the target is judged, so a
	// resolved and an unresolved call alike answer with the argument's error.
	for _, src := range []string{"twice(9223372036854775808)", "nowhere(9223372036854775808)"} {
		expr := parseExpr(t, src)
		for i := 0; i < 2; i++ {
			if _, err := ec.Eval(expr); !errors.Is(err, semantics.ErrArithmeticOverflow) {
				t.Fatalf("eval %d of %s: err = %v; want the argument's ErrArithmeticOverflow", i, src, err)
			}
		}
	}
}
